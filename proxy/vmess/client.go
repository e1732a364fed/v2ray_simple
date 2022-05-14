package vmess

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/crypto/chacha20poly1305"
)

func init() {
	//proxy.RegisterClient(Name, NewVmessClient)
}

func NewVmessClient(url *url.URL) (*Client, error) {
	addr := url.Host
	uuidStr := url.User.Username()
	uuid, err := utils.StrToUUID(uuidStr)
	if err != nil {
		return nil, err
	}

	query := url.Query()

	security := query.Get("security")
	if security == "" {
		security = "none"
	}

	c := &Client{}
	c.SetAddrStr(addr)
	user := utils.V2rayUser(uuid)
	c.user = user

	c.opt = OptChunkStream
	security = strings.ToLower(security)
	switch security {
	case "aes-128-gcm":
		c.security = SecurityAES128GCM
	case "chacha20-poly1305":
		c.security = SecurityChacha20Poly1305
	case "none":
		c.security = SecurityNone
	case "":
		// NOTE: use basic format when no method specified
		c.opt = OptBasicFormat
		c.security = SecurityNone
	default:
		return nil, errors.New("unknown security type: " + security)
	}
	rand.Seed(time.Now().UnixNano())

	return c, nil
}

// Client is a vmess client
type Client struct {
	proxy.Base
	user     utils.V2rayUser
	opt      byte
	security byte
}

func (c *Client) Name() string { return Name }

func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (io.ReadWriter, error) {

	conn := &ClientConn{user: c.user, opt: c.opt, security: c.security}
	conn.Conn = underlay

	conn.addr, conn.atyp = target.AddressBytes()
	conn.port = uint16(target.Port)

	randBytes := utils.GetBytes(32)
	rand.Read(randBytes)
	copy(conn.reqBodyIV[:], randBytes[:16])
	copy(conn.reqBodyKey[:], randBytes[16:32])
	utils.PutBytes(randBytes)
	conn.reqRespV = byte(rand.Intn(1 << 8))
	conn.respBodyIV = md5.Sum(conn.reqBodyIV[:])
	conn.respBodyKey = md5.Sum(conn.reqBodyKey[:])

	// Auth
	err := conn.auth()
	if err != nil {
		return nil, err
	}

	// Request
	err = conn.handshake(CmdTCP)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// ClientConn is a connection to vmess server
type ClientConn struct {
	net.Conn

	user     utils.V2rayUser
	opt      byte
	security byte

	atyp byte
	addr []byte
	port uint16

	reqBodyIV   [16]byte
	reqBodyKey  [16]byte
	reqRespV    byte
	respBodyIV  [16]byte
	respBodyKey [16]byte

	dataReader io.Reader
	dataWriter io.Writer
}

// send auth info: HMAC("md5", UUID, UTC)
func (c *ClientConn) auth() error {
	ts := utils.GetBytes(8)
	defer utils.PutBytes(ts)

	binary.BigEndian.PutUint64(ts, uint64(time.Now().UTC().Unix()))

	h := hmac.New(md5.New, c.user.IdentityBytes())
	h.Write(ts)

	_, err := c.Conn.Write(h.Sum(nil))
	return err
}

// handshake sends request to server.
func (c *ClientConn) handshake(cmd byte) error {
	buf := utils.GetBuf()
	defer utils.PutBuf(buf)

	// Request
	buf.WriteByte(1)           // Ver
	buf.Write(c.reqBodyIV[:])  // IV
	buf.Write(c.reqBodyKey[:]) // Key
	buf.WriteByte(c.reqRespV)  // V
	buf.WriteByte(c.opt)       // Opt

	// pLen and Sec
	paddingLen := rand.Intn(16)
	pSec := byte(paddingLen<<4) | c.security // P(4bit) and Sec(4bit)
	buf.WriteByte(pSec)

	buf.WriteByte(0) // reserved
	buf.WriteByte(cmd)

	// target
	err := binary.Write(buf, binary.BigEndian, c.port) // port
	if err != nil {
		return err
	}

	buf.WriteByte(c.atyp) // atyp
	buf.Write(c.addr)     // addr

	// padding
	if paddingLen > 0 {
		padding := utils.GetBytes(paddingLen)
		rand.Read(padding)
		buf.Write(padding)
		utils.PutBytes(padding)
	}

	// F
	fnv1a := fnv.New32a()
	_, err = fnv1a.Write(buf.Bytes())
	if err != nil {
		return err
	}
	buf.Write(fnv1a.Sum(nil))

	// log.Printf("Request Send %v", buf.Bytes())

	block, err := aes.NewCipher(GetKey(c.user))
	if err != nil {
		return err
	}

	stream := cipher.NewCFBEncrypter(block, TimestampHash(time.Now().UTC().Unix()))
	stream.XORKeyStream(buf.Bytes(), buf.Bytes())

	_, err = c.Conn.Write(buf.Bytes())

	return err
}

// DecodeRespHeader decodes response header.
func (c *ClientConn) DecodeRespHeader() error {
	block, err := aes.NewCipher(c.respBodyKey[:])
	if err != nil {
		return err
	}

	stream := cipher.NewCFBDecrypter(block, c.respBodyIV[:])

	b := utils.GetBytes(4)
	defer utils.PutBytes(b)

	_, err = io.ReadFull(c.Conn, b)
	if err != nil {
		return err
	}

	stream.XORKeyStream(b, b)

	if b[0] != c.reqRespV {
		return errors.New("unexpected response header")
	}

	if b[2] != 0 {
		// dataLen := int32(buf[3])
		return errors.New("dynamic port is not supported now")
	}

	return nil
}

func (c *ClientConn) Write(b []byte) (n int, err error) {
	if c.dataWriter != nil {
		return c.dataWriter.Write(b)
	}

	c.dataWriter = c.Conn
	if c.opt&OptChunkStream == OptChunkStream {
		switch c.security {
		case SecurityNone:
			c.dataWriter = ChunkedWriter(c.Conn)

		case SecurityAES128GCM:
			block, _ := aes.NewCipher(c.reqBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataWriter = AEADWriter(c.Conn, aead, c.reqBodyIV[:])

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.reqBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataWriter = AEADWriter(c.Conn, aead, c.reqBodyIV[:])
			utils.PutBytes(key)
		}
	}

	return c.dataWriter.Write(b)
}

func (c *ClientConn) Read(b []byte) (n int, err error) {
	if c.dataReader != nil {
		return c.dataReader.Read(b)
	}

	err = c.DecodeRespHeader()
	if err != nil {
		return 0, err
	}

	c.dataReader = c.Conn
	if c.opt&OptChunkStream == OptChunkStream {
		switch c.security {
		case SecurityNone:
			c.dataReader = ChunkedReader(c.Conn)

		case SecurityAES128GCM:
			block, _ := aes.NewCipher(c.respBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataReader = AEADReader(c.Conn, aead, c.respBodyIV[:])

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.respBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataReader = AEADReader(c.Conn, aead, c.respBodyIV[:])
			utils.PutBytes(key)
		}
	}

	return c.dataReader.Read(b)
}
