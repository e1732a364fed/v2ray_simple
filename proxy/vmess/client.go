package vmess

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/crypto/chacha20poly1305"
)

const systemAutoWillUseAes = runtime.GOARCH == "amd64" || runtime.GOARCH == "s390x" || runtime.GOARCH == "arm64"

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

type ClientCreator struct{}

func (ClientCreator) NewClientFromURL(url *url.URL) (proxy.Client, error) {
	uuidStr := url.User.Username()
	uuid, err := utils.StrToUUID(uuidStr)
	if err != nil {
		return nil, err
	}

	query := url.Query()

	security := query.Get("security")

	c := &Client{}
	c.user = utils.V2rayUser(uuid)

	c.opt = OptChunkStream
	if err = c.specifySecurityByStr(security); err != nil {
		return nil, err
	}

	return c, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {
	uuid, err := utils.StrToUUID(dc.Uuid)
	if err != nil {
		return nil, err
	}
	c := &Client{}
	c.user = utils.V2rayUser(uuid)
	c.opt = OptChunkStream

	if len(dc.Extra) > 0 {
		if thing := dc.Extra["vmess_security"]; thing != nil {
			if str, ok := thing.(string); ok {
				if err = c.specifySecurityByStr(str); err != nil {
					return nil, err
				}

			}
		}
	} else {
		c.specifySecurityByStr("")
	}

	return c, nil
}

type Client struct {
	proxy.Base
	user     utils.V2rayUser
	opt      byte
	security byte
}

func (c *Client) specifySecurityByStr(security string) error {
	security = strings.ToLower(security)
	switch security {
	case "aes-128-gcm":
		c.security = SecurityAES128GCM
	case "chacha20-poly1305":
		c.security = SecurityChacha20Poly1305
	case "auto":
		if systemAutoWillUseAes {
			c.security = SecurityAES128GCM
		} else {
			c.security = SecurityChacha20Poly1305

		}
	case "none":
		c.security = SecurityNone

	case "", "zero": // NOTE: use basic format when no method specified.

		c.opt = OptBasicFormat
		c.security = SecurityNone
	default:
		return errors.New("unknown security type: " + security)
	}
	return nil
}

func (c *Client) Name() string { return Name }

func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (io.ReadWriteCloser, error) {

	return c.commonHandshake(underlay, firstPayload, target)
}

func (c *Client) EstablishUDPChannel(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (netLayer.MsgConn, error) {
	return c.commonHandshake(underlay, firstPayload, target)

}

func (c *Client) commonHandshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (*ClientConn, error) {

	conn := &ClientConn{user: c.user, opt: c.opt, security: c.security}
	conn.Conn = underlay

	conn.addr, conn.atyp = target.AddressBytes()
	conn.port = uint16(target.Port)

	randBytes := utils.GetBytes(33)
	rand.Read(randBytes)
	copy(conn.reqBodyIV[:], randBytes[:16])
	copy(conn.reqBodyKey[:], randBytes[16:32])
	utils.PutBytes(randBytes)
	conn.reqRespV = randBytes[32]

	bodyKey := sha256.Sum256(conn.reqBodyKey[:])
	bodyIV := sha256.Sum256(conn.reqBodyIV[:])
	copy(conn.respBodyKey[:], bodyKey[:16])
	copy(conn.respBodyIV[:], bodyIV[:16])

	var err error

	// Request
	if target.IsUDP() {
		err = conn.handshake(CmdUDP)
		conn.theTarget = target

	} else {
		err = conn.handshake(CmdTCP)

	}

	if err != nil {
		return nil, err
	}
	if len(firstPayload) > 0 {
		_, err = conn.Write(firstPayload)
	}

	return conn, err
}

// ClientConn is a connection to vmess server
type ClientConn struct {
	net.Conn

	user     utils.V2rayUser
	opt      byte
	security byte

	theTarget netLayer.Addr

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

func (c *ClientConn) CloseConnWithRaddr(_ netLayer.Addr) error {
	return c.Conn.Close()
}

//vmess 标准 是不支持 fullcone的，和vless v0相同
func (c *ClientConn) Fullcone() bool {
	return false
}

func (c *ClientConn) ReadMsgFrom() (bs []byte, target netLayer.Addr, err error) {
	bs = utils.GetPacket()
	var n int
	n, err = c.Read(bs)
	if err != nil {
		utils.PutPacket(bs)
		bs = nil
		return
	}
	bs = bs[:n]
	target = c.theTarget
	return
}

func (c *ClientConn) WriteMsgTo(b []byte, _ netLayer.Addr) error {
	_, e := c.Write(b)
	return e
}

// handshake sends request to server.
func (c *ClientConn) handshake(cmd byte) error {
	buf := utils.GetBuf()
	defer utils.PutBuf(buf)

	// Request
	buf.WriteByte(1) // Ver
	buf.Write(c.reqBodyIV[:])
	buf.Write(c.reqBodyKey[:])
	buf.WriteByte(c.reqRespV)
	buf.WriteByte(c.opt)

	// pLen and Sec
	paddingLen := rand.Intn(16)
	pSec := byte(paddingLen<<4) | c.security // P(4bit) and Sec(4bit)
	buf.WriteByte(pSec)

	buf.WriteByte(0) // reserved
	buf.WriteByte(cmd)

	// target
	err := binary.Write(buf, binary.BigEndian, c.port)
	if err != nil {
		return err
	}

	buf.WriteByte(c.atyp)
	buf.Write(c.addr)

	// padding
	if paddingLen > 0 {
		padding := utils.GetBytes(paddingLen)
		rand.Read(padding)
		buf.Write(padding)
		utils.PutBytes(padding)
	}

	fnv1a := fnv.New32a()
	_, err = fnv1a.Write(buf.Bytes())
	if err != nil {
		return err
	}
	buf.Write(fnv1a.Sum(nil))

	var fixedLengthCmdKey [16]byte
	copy(fixedLengthCmdKey[:], GetKey(c.user))
	vmessout := sealVMessAEADHeader(fixedLengthCmdKey, buf.Bytes(), time.Now())
	_, err = c.Conn.Write(vmessout)
	return err

}

func (vc *ClientConn) aead_decodeRespHeader() error {
	var buf []byte
	aeadResponseHeaderLengthEncryptionKey := kdf(vc.respBodyKey[:], kdfSaltConstAEADRespHeaderLenKey)[:16]
	aeadResponseHeaderLengthEncryptionIV := kdf(vc.respBodyIV[:], kdfSaltConstAEADRespHeaderLenIV)[:12]

	aeadResponseHeaderLengthEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderLengthEncryptionKey)
	aeadResponseHeaderLengthEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderLengthEncryptionKeyAESBlock)

	aeadEncryptedResponseHeaderLength := make([]byte, 18)
	if _, err := io.ReadFull(vc.Conn, aeadEncryptedResponseHeaderLength); err != nil {
		return err
	}

	decryptedResponseHeaderLengthBinaryBuffer, err := aeadResponseHeaderLengthEncryptionAEAD.Open(nil, aeadResponseHeaderLengthEncryptionIV, aeadEncryptedResponseHeaderLength[:], nil)
	if err != nil {
		return err
	}
	decryptedResponseHeaderLength := binary.BigEndian.Uint16(decryptedResponseHeaderLengthBinaryBuffer)
	aeadResponseHeaderPayloadEncryptionKey := kdf(vc.respBodyKey[:], kdfSaltConstAEADRespHeaderPayloadKey)[:16]
	aeadResponseHeaderPayloadEncryptionIV := kdf(vc.respBodyIV[:], kdfSaltConstAEADRespHeaderPayloadIV)[:12]
	aeadResponseHeaderPayloadEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderPayloadEncryptionKey)
	aeadResponseHeaderPayloadEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderPayloadEncryptionKeyAESBlock)

	encryptedResponseHeaderBuffer := make([]byte, decryptedResponseHeaderLength+16)
	if _, err := io.ReadFull(vc.Conn, encryptedResponseHeaderBuffer); err != nil {
		return err
	}

	buf, err = aeadResponseHeaderPayloadEncryptionAEAD.Open(nil, aeadResponseHeaderPayloadEncryptionIV, encryptedResponseHeaderBuffer, nil)
	if err != nil {
		return err
	}

	if len(buf) < 4 {
		return errors.New("unexpected buffer length")
	}

	if buf[0] != vc.reqRespV {
		return errors.New("unexpected response header")
	}

	if buf[2] != 0 {
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

	err = c.aead_decodeRespHeader()

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
