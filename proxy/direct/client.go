// Package direct provies a struct that implements proxy.Client
package direct

import (
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

const name = "direct"

func init() {
	proxy.RegisterClient(name, &ClientCreator{})
}

//实现了 proxy.Client, netLayer.UDP_Putter_Generator
type Client struct {
	proxy.ProxyCommonStruct
	isfullcone bool
}

type ClientCreator struct{}

func (_ ClientCreator) NewClientFromURL(*url.URL) (proxy.Client, error) {
	d := &Client{}
	return d, nil
}

func (_ ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {
	d := &Client{}
	d.isfullcone = dc.Fullcone
	return d, nil
}

func (d *Client) Name() string { return name }

//若 underlay 为nil，则我们会自动对target进行拨号。
func (d *Client) Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error) {

	if underlay == nil {
		return target.Dial()
	}

	return underlay, nil

}

//direct的Client的 EstablishUDPChannel 实际上就是直接 监听一个udp端口。
func (d *Client) EstablishUDPChannel(_ net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {

	return netLayer.NewUDPMsgConnClientWrapper(nil, d.isfullcone, false), nil
}
