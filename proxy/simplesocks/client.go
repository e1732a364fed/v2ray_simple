package simplesocks

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

type ClientCreator struct{ proxy.CreatorCommonStruct }

func (ClientCreator) UseUDPAsMsgConn() bool {
	return false
}

func (ClientCreator) URLToDialConf(u *url.URL, dc *proxy.DialConf, format int) (*proxy.DialConf, error) {
	if dc == nil {
		dc = &proxy.DialConf{}
	}
	return dc, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	return &Client{}, nil
}

type Client struct {
	proxy.Base
}

func (*Client) GetCreator() proxy.ClientCreator {
	return ClientCreator{}
}
func (c *Client) Name() string {
	return Name
}

func WriteAddrToBuf(target netLayer.Addr, buf *bytes.Buffer) {
	if len(target.IP) > 0 {
		if ip4 := target.IP.To4(); ip4 == nil {
			buf.WriteByte(ATypIP6)
			buf.Write(target.IP)
		} else {
			buf.WriteByte(ATypIP4)
			buf.Write(ip4)
		}
	} else if l := len(target.Name); l > 0 {
		buf.WriteByte(ATypDomain)
		buf.WriteByte(byte(l))
		buf.WriteString(target.Name)
	}

	buf.WriteByte(byte(target.Port >> 8))
	buf.WriteByte(byte(target.Port << 8 >> 8))
}

func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (io.ReadWriteCloser, error) {
	if target.Port <= 0 {
		return nil, errors.New("simplesocks Client Handshake failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.WriteByte(CmdTCP)
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

	return &TCPConn{
		Conn:            underlay,
		underlayIsBasic: netLayer.IsBasicConn(underlay),
	}, nil
}

func (c *Client) EstablishUDPChannel(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (netLayer.MsgConn, error) {
	if target.Port <= 0 {
		return nil, errors.New("simplesocks Client EstablishUDPChannel failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.WriteByte(CmdUDP)
	WriteAddrToBuf(target, buf)

	uc := NewUDPConn(underlay, nil)
	uc.handshakeBuf = buf
	uc.fullcone = c.IsFullcone

	if len(firstPayload) == 0 {
		return uc, nil

	} else {
		return uc, uc.WriteMsgTo(firstPayload, target)
	}
}
