package shadowsocks

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/shadowsocks/go-shadowsocks2/core"
)

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

type ClientCreator struct{ proxy.CreatorCommonStruct }

func (ClientCreator) UseUDPAsMsgConn() bool {
	return true
}
func (ClientCreator) MultiTransportLayer() bool {
	return true
}
func (ClientCreator) URLToDialConf(u *url.URL, dc *proxy.DialConf, format int) (*proxy.DialConf, error) {
	if format != proxy.UrlStandardFormat {
		return dc, utils.ErrUnImplemented
	}
	if dc == nil {
		dc = &proxy.DialConf{}
	}

	if p, set := u.User.Password(); set {

		dc.Uuid = "method:" + u.User.Username() + "\npass:" + p
	}

	return dc, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	uuidStr := dc.Uuid
	var mp MethodPass
	if mp.InitWithStr(uuidStr) {
		return newClient(mp), nil

	}

	return nil, utils.ErrNilOrWrongParameter
}

type Client struct {
	proxy.Base
	utils.UserPass
	cipher core.Cipher
}

func newClient(mp MethodPass) *Client {
	return &Client{
		cipher: initShadowCipher(mp),
	}
}
func (*Client) GetCreator() proxy.ClientCreator {
	return ClientCreator{}
}
func (*Client) Name() string {
	return Name
}
func (c *Client) Network() string {
	if c.TransportLayer == "" {
		return netLayer.DualNetworkName
	} else {
		return c.TransportLayer
	}
}
func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (conn io.ReadWriteCloser, err error) {
	conn = c.cipher.StreamConn(underlay)

	buf := utils.GetBuf()
	defer utils.PutBuf(buf)
	abs, atype := target.AddressBytes()
	buf.WriteByte(netLayer.ATypeToSocks5Standard(atype))
	buf.Write(abs)
	buf.WriteByte(byte(target.Port >> 8))
	buf.WriteByte(byte(target.Port << 8 >> 8))
	if len(firstPayload) > 0 {
		buf.Write(firstPayload)
	}
	_, err = conn.Write(buf.Bytes())

	return
}

func (c *Client) EstablishUDPChannel(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (mc netLayer.MsgConn, err error) {
	var ok bool
	var pc net.PacketConn

	if underlay != nil {
		pc, ok = underlay.(net.PacketConn)
	}

	if !ok {
		pc, err = c.Base.DialUDP(target)
		if err != nil {
			return
		}
	}

	if c.cipher != nil {
		pc = c.cipher.PacketConn(pc)
	}

	mc = &clientUDPMsgConn{
		PacketConn: pc,
		raddr:      underlay.RemoteAddr(),
	}

	if firstPayload != nil {
		err = mc.WriteMsg(firstPayload, target)
	}

	return

}
