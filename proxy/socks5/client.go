package socks5

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	proxy.RegisterClient(Name, &ClientCreator{})
}

type ClientCreator struct{}

func (ClientCreator) NewClientFromURL(*url.URL) (proxy.Client, error) {
	s := &Client{}
	return s, nil
}

func (ClientCreator) NewClient(*proxy.DialConf) (proxy.Client, error) {
	s := &Client{}
	return s, nil
}

type Client struct {
	proxy.Base
}

func (*Client) Name() string {
	return Name
}

func (*Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (result io.ReadWriteCloser, err error) {

	if underlay == nil {
		panic("socks5 client handshake, nil underlay is not allowed")
	}

	var ba [10]byte

	//握手阶段
	ba[0] = Version5
	ba[1] = 1
	ba[2] = 0
	_, err = underlay.Write(ba[:3])
	if err != nil {
		return
	}

	n, err := underlay.Read(ba[:])
	if err != nil {
		return
	}

	if n != 2 || ba[0] != Version5 || ba[1] != 0 {
		return nil, utils.NumErr{Prefix: "socks5 client handshake,protocol err", N: 1}
	}

	buf := utils.GetBuf()
	buf.WriteByte(Version5)
	buf.WriteByte(CmdConnect)
	buf.WriteByte(0)
	abs, atype := target.AddressBytes()

	buf.WriteByte(netLayer.ATypeToSocks5Standard(atype))
	buf.Write(abs)
	buf.WriteByte(byte(target.Port >> 8))
	buf.WriteByte(byte(target.Port << 8 >> 8))

	n, err = underlay.Read(ba[:])
	if err != nil {
		return
	}
	if n < 10 || ba[0] != 5 || ba[1] != 0 || ba[2] != 0 {
		return nil, utils.NumErr{Prefix: "socks5 client handshake failed when reading response", N: 2}

	}
	if len(firstPayload) > 0 {
		underlay.Write(firstPayload)
	}

	return underlay, nil

}

func (c *Client) EstablishUDPChannel(underlay net.Conn, _ netLayer.Addr) (netLayer.MsgConn, error) {
	var err error
	serverPort := 0
	serverPort, err = Client_EstablishUDPAssociate(underlay)
	if err != nil {
		return nil, err
	}

	ua, err := net.ResolveUDPAddr("udp", c.Addr)
	if err != nil {
		return nil, err
	}
	cpc := ClientUDPConn{
		associated:          true,
		ServerUDPPort_forMe: serverPort,
		ServerAddr: &net.TCPAddr{
			IP: ua.IP,
		},
	}
	cpc.UDPConn, err = net.DialUDP("udp", nil, ua)
	if err != nil {
		return nil, err
	}

	return &cpc, nil
}
