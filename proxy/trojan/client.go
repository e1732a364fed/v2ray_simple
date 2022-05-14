package trojan

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

//作为对照，可以参考 https://github.com/p4gefau1t/trojan-go/blob/master/tunnel/trojan/client.go

type ClientCreator struct{}

func (ClientCreator) NewClientFromURL(url *url.URL) (proxy.Client, error) {
	uuidStr := url.User.Username()
	c := Client{
		User: NewUserByPlainTextPassword(uuidStr),
	}

	return &c, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	uuidStr := dc.Uuid

	c := Client{
		use_mux: dc.Mux,
		User:    NewUserByPlainTextPassword(uuidStr),
	}

	return &c, nil
}

type Client struct {
	proxy.Base
	User
	use_mux bool
}

func (*Client) Name() string {
	return Name
}

func (c *Client) HasInnerMux() (int, string) {
	if c.use_mux {
		return 2, "simplesocks"

	} else {
		return 0, ""

	}
}

func (c *Client) GetUser() utils.User {
	return c.User
}

func WriteAddrToBuf(target netLayer.Addr, buf *bytes.Buffer) {
	if len(target.IP) > 0 {
		if ip4 := target.IP.To4(); ip4 == nil {
			buf.WriteByte(netLayer.AtypIP6)
			buf.Write(target.IP)
		} else {
			buf.WriteByte(netLayer.AtypIP4)
			buf.Write(ip4)
		}
	} else if l := len(target.Name); l > 0 {
		buf.WriteByte(ATypDomain)
		buf.WriteByte(byte(l))
		buf.WriteString(target.Name)
	}

	buf.WriteByte(byte(target.Port >> 8))
	buf.WriteByte(byte(target.Port << 8 >> 8))
	buf.Write(crlf)
}

func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (io.ReadWriteCloser, error) {
	if target.Port <= 0 {
		return nil, errors.New("trojan Client Handshake failed, target port invalid")

	}
	buf := utils.GetBuf()

	buf.WriteString(c.AuthStr())
	buf.Write(crlf)
	if c.use_mux {
		buf.WriteByte(CmdMux)

	} else {
		buf.WriteByte(CmdConnect)

	}
	WriteAddrToBuf(target, buf)
	if len(firstPayload) > 0 {
		buf.Write(firstPayload)
		utils.PutBytes(firstPayload)
	}
	_, err := underlay.Write(buf.Bytes())
	utils.PutBuf(buf)
	if err != nil {
		return nil, err
	}

	if c.use_mux {
		// we return underlay directly, the caller can call HasInnerMux method to see whether we use innerMux or not.
		return underlay, nil
	} else {
		// 发现直接返回 underlay 反倒无法利用readv, 所以还是统一用包装过的. 目前利用readv是可以加速的.
		return &UserTCPConn{
			Conn:            underlay,
			User:            c.User,
			underlayIsBasic: netLayer.IsBasicConn(underlay),
		}, nil
	}

}

func (c *Client) EstablishUDPChannel(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (netLayer.MsgConn, error) {
	if target.Port <= 0 {
		return nil, errors.New("trojan Client EstablishUDPChannel failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.WriteString(c.AuthStr())
	buf.Write(crlf)
	buf.WriteByte(CmdUDPAssociate)
	WriteAddrToBuf(target, buf)

	uc := NewUDPConn(underlay, nil)
	uc.User = c.User
	uc.handshakeBuf = buf
	if len(firstPayload) == 0 {
		return uc, nil
	} else {
		return uc, uc.WriteMsgTo(firstPayload, target)
	}
}
