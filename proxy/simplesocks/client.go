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

type ClientCreator struct{}

func (ClientCreator) NewClientFromURL(u *url.URL) (proxy.Client, error) {

	return &Client{}, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	return &Client{}, nil
}

type Client struct {
	proxy.Base
}

func (c *Client) Name() string {
	return Name
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

func (c *Client) EstablishUDPChannel(underlay net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {
	if target.Port <= 0 {
		return nil, errors.New("simplesocks Client EstablishUDPChannel failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.WriteByte(CmdUDP)
	WriteAddrToBuf(target, buf)

	uc := NewUDPConn(underlay, nil)
	uc.handshakeBuf = buf
	return uc, nil
}
