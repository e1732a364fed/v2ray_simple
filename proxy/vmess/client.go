package vmess

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/kem/mceliece/mceliece8192128f"
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
	if len(dc.Extra) > 0 {
		if thing := dc.Extra["server_publickey"]; thing != nil {
			if str, ok := thing.(string); ok {
				ds, err := hex.DecodeString(str)
				if err != nil {
					return nil, err
				}
				pub, err := mceliece8192128f.Scheme().UnmarshalBinaryPublicKey(ds)
				if err != nil {
					return nil, err
				}
				c.srvpub = pub
			}
		}
	}
	return c, nil
}

type Client struct {
	proxy.Base
	utils.V2rayUser
	srvpub kem.PublicKey

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
	case "aes-256-gcm":
		c.security = SecurityAES256GCM
	case "chacha20-poly1305":
		c.security = SecurityChacha20Poly1305
	case "auto", "": //这里我们为了保护用户，当字符串为空时，依然设为auto，而不是zero
		if utils.SystemAutoUseAes {
			c.security = SecurityAES256GCM
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
		user: c.V2rayUser,

		Conn:     underlay,
		opt:      c.opt,
		security: c.security,
		port:     uint16(target.Port),
		pub:      c.srvpub,
	}

	conn.addr, conn.atyp = target.AddressBytes()

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

	opt      byte
	security byte
	user     utils.V2rayUser

	theTarget netLayer.Addr

	atyp byte
	addr []byte
	port uint16

	pub          kem.PublicKey
	sharedsecret []byte

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

	result := utils.GetBuf()
	defer utils.PutBuf(result)

	ct, ss, err := mceliece8192128f.Scheme().Encapsulate(c.pub)
	if err != nil {
		return err
	}

	c.sharedsecret = ss
	result.Write(ct)
	buf.WriteByte(c.opt)
	buf.WriteByte(c.security)
	buf.WriteByte(cmd)

	// target
	err = binary.Write(buf, binary.BigEndian, c.port)
	if err != nil {
		return err
	}
	buf.WriteByte(c.atyp)
	buf.Write(c.addr)
	result.Write(sealAEADHeader(c.sharedsecret, buf.Bytes(), time.Now()))
	c.vmessout = result.Bytes()

	_, err = c.Write(firstpayload)

	return err

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
		c2s := kdf(c.sharedsecret, []byte(kdfSaltConstAEADKEY), []byte("Client2Server"))
		switch c.security {
		case SecurityNone:
			c.dataWriter = ChunkedWriter(c.dataWriter)
		case SecurityAES256GCM:
			block, _ := aes.NewCipher(c2s[:chacha20poly1305.KeySize])
			aead, _ := cipher.NewGCM(block)
			c.dataWriter = AEADWriter(c.dataWriter, aead, c2s[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		case SecurityChacha20Poly1305:
			aead, _ := chacha20poly1305.New(c2s[:chacha20poly1305.KeySize])
			c.dataWriter = AEADWriter(c.dataWriter, aead, c2s[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
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
		// if err != nil {
		// 	return 0, err
		// }
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
	// err = c.aead_decodeRespHeader()
	// if err != nil {
	// 	return 0, err
	// }
	c.dataReader = c.Conn
	if c.opt&OptChunkStream > 0 {
		s2c := kdf(c.sharedsecret, []byte(kdfSaltConstAEADKEY), []byte("Server2Client"))
		switch c.security {
		case SecurityNone:
			c.dataReader = ChunkedReader(c.Conn)
		case SecurityAES256GCM:
			block, _ := aes.NewCipher(s2c[:chacha20poly1305.KeySize])
			aead, _ := cipher.NewGCM(block)
			c.dataReader = AEADReader(c.Conn, aead, s2c[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		case SecurityChacha20Poly1305:
			aead, _ := chacha20poly1305.New(s2c[:chacha20poly1305.KeySize])
			c.dataReader = AEADReader(c.Conn, aead, s2c[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		}
	}

	return c.dataReader.Read(b)
}
