package vmess

import (
	"bytes"
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
	"strings"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/crypto/chacha20poly1305"
)

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

const Security_confStr string = "vmess_security"

func GetEncryptAlgo(dc *proxy.DialConf) (result string) {

	if len(dc.Extra) > 0 {
		if thing := dc.Extra[Security_confStr]; thing != nil {
			if str, ok := thing.(string); ok {

				result = str

			}
		}
	}

	if dc.EncryptAlgo != "" {
		result = dc.EncryptAlgo
	}

	return result
}

type ClientCreator struct{ proxy.CreatorCommonStruct }

func (ClientCreator) URLToDialConf(url *url.URL, dc *proxy.DialConf, format int) (*proxy.DialConf, error) {
	if format != proxy.UrlStandardFormat {
		return dc, utils.ErrUnImplemented
	}

	if dc == nil {
		dc = &proxy.DialConf{}
		uuidStr := url.User.Username()

		dc.Uuid = uuidStr

	}

	return dc, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {
	uuid, err := utils.StrToUUID(dc.Uuid)
	if err != nil {
		return nil, err
	}
	c := &Client{
		use_mux: dc.Mux,
	}
	c.V2rayUser = utils.V2rayUser(uuid)
	c.opt = OptChunkStream

	var ea string = GetEncryptAlgo(dc)

	if err := c.specifySecurityByStr(ea); err != nil {

		return nil, err
	}

	return c, nil
}

type Client struct {
	proxy.Base
	utils.V2rayUser

	opt      byte
	security byte
	use_mux  bool
}

func (*Client) GetCreator() proxy.ClientCreator {
	return ClientCreator{}
}

func (c *Client) HasInnerMux() (int, string) {
	if c.use_mux {
		return 2, "simplesocks"

	} else {
		return 0, ""

	}
}

func (c *Client) specifySecurityByStr(security string) error {
	security = strings.ToLower(security)
	switch security {
	case "aes-128-gcm":
		c.security = SecurityAES128GCM
	case "chacha20-poly1305":
		c.security = SecurityChacha20Poly1305
	case "auto", "": //这里我们为了保护用户，当字符串为空时，依然设为auto，而不是zero
		if utils.SystemAutoUseAes {
			c.security = SecurityAES128GCM
		} else {
			c.security = SecurityChacha20Poly1305

		}
	case "none":
		c.security = SecurityNone

	case "zero", "0":

		// NOTE: use basic format when no method specified.
		// 注意，BasicFormat 只用于向前兼容，本作的vmess的服务端并不支持 注意，BasicFormat

		c.opt = OptBasicFormat
		c.security = SecurityNone
	default:
		return utils.ErrInErr{ErrDesc: "unknown security type", ErrDetail: utils.ErrInvalidData, Data: security}
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

	conn := &ClientConn{
		V2rayUser: c.V2rayUser,
		Conn:      underlay,
		opt:       c.opt,
		security:  c.security,
		port:      uint16(target.Port),
	}

	conn.addr, conn.atyp = target.AddressBytes()

	randBytes := utils.GetBytes(33)
	rand.Read(randBytes)
	copy(conn.reqBodyIV[:], randBytes[:16])
	copy(conn.reqBodyKey[:], randBytes[16:32])
	conn.reqRespV = randBytes[32]
	utils.PutBytes(randBytes)

	bodyKey := sha256.Sum256(conn.reqBodyKey[:])
	bodyIV := sha256.Sum256(conn.reqBodyIV[:])
	copy(conn.respBodyKey[:], bodyKey[:16])
	copy(conn.respBodyIV[:], bodyIV[:16])

	var err error

	if c.use_mux {
		err = conn.handshake(CMDMux_VS, firstPayload)
		conn.use_mux = true

	} else {
		// Request
		if target.IsUDP() {
			err = conn.handshake(CmdUDP, firstPayload)
			conn.theTarget = target

		} else {
			err = conn.handshake(CmdTCP, firstPayload)

		}
	}

	if err != nil {
		return nil, err
	}

	return conn, err
}

// ClientConn is a connection to vmess server
type ClientConn struct {
	net.Conn

	utils.V2rayUser
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

	vmessout []byte

	use_mux bool
	RM      sync.Mutex //用于mux，因为可能同一时间多个写入发生
	WM      sync.Mutex
}

func (c *ClientConn) CloseConnWithRaddr(_ netLayer.Addr) error {
	return c.Conn.Close()
}

// return false; vmess 标准 是不支持 fullcone的，和vless v0相同
func (c *ClientConn) Fullcone() bool {
	return false
}

func (c *ClientConn) ReadMsg() (bs []byte, target netLayer.Addr, err error) {
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

func (c *ClientConn) WriteMsg(b []byte, _ netLayer.Addr) error {
	_, e := c.Write(b)
	return e
}

// handshake sends request to server.
func (c *ClientConn) handshake(cmd byte, firstpayload []byte) error {
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

	c.vmessout = sealAEADHeader(GetKey(c.V2rayUser), buf.Bytes(), time.Now())

	_, err = c.Write(firstpayload)

	return err

}

func (vc *ClientConn) aead_decodeRespHeader() error {
	var buf []byte
	aeadResponseHeaderLengthEncryptionKey := kdf16(vc.respBodyKey[:], kdfSaltConstAEADRespHeaderLenKey)
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
		return errors.New("vless aead_decodeRespHeader unexpected buffer length")
	}

	if buf[0] != vc.reqRespV {
		return errors.New("vless aead_decodeRespHeader unexpected response header")
	}

	if buf[2] != 0 {
		return errors.New("vless aead_decodeRespHeader, dynamic port is not supported now")
	}

	return nil
}

func (c *ClientConn) Write(b []byte) (n int, err error) {
	if c.use_mux {
		c.WM.Lock()
		defer c.WM.Unlock()
	}
	if c.dataWriter != nil {
		return c.dataWriter.Write(b)
	}
	c.dataWriter = c.Conn

	switchChan := make(chan struct{})
	var outBuf *bytes.Buffer
	if len(b) == 0 {
		_, err = c.Conn.Write(c.vmessout)
		c.vmessout = nil
		if err != nil {
			return 0, err
		}
	} else {
		outBuf = bytes.NewBuffer(c.vmessout)
		writer := &utils.WriteSwitcher{
			Old:        outBuf,
			New:        c.Conn,
			SwitchChan: switchChan,
			Closer:     c.Conn,
		}

		c.dataWriter = writer
	}

	if c.opt&OptChunkStream > 0 {
		switch c.security {
		case SecurityNone:
			c.dataWriter = ChunkedWriter(c.dataWriter)

		case SecurityAES128GCM:
			block, _ := aes.NewCipher(c.reqBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataWriter = AEADWriter(c.dataWriter, aead, c.reqBodyIV[:], nil)

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.reqBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataWriter = AEADWriter(c.dataWriter, aead, c.reqBodyIV[:], nil)
			utils.PutBytes(key)
		}
	}

	if len(b) > 0 {
		n, err = c.dataWriter.Write(b)
		close(switchChan)
		c.vmessout = nil

		if err != nil {
			return
		}
		_, err = c.Conn.Write(outBuf.Bytes())
	}

	return
}

func (c *ClientConn) Read(b []byte) (n int, err error) {

	if c.use_mux {
		c.RM.Lock()
		defer c.RM.Unlock()
	}

	if c.dataReader != nil {
		return c.dataReader.Read(b)
	}

	err = c.aead_decodeRespHeader()

	if err != nil {
		return 0, err
	}

	c.dataReader = c.Conn
	if c.opt&OptChunkStream > 0 {
		switch c.security {
		case SecurityNone:
			c.dataReader = ChunkedReader(c.Conn)

		case SecurityAES128GCM:
			block, _ := aes.NewCipher(c.respBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataReader = AEADReader(c.Conn, aead, c.respBodyIV[:], nil)

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.respBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataReader = AEADReader(c.Conn, aead, c.respBodyIV[:], nil)
			utils.PutBytes(key)
		}
	}

	return c.dataReader.Read(b)
}
