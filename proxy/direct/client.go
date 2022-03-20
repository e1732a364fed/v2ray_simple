// Package direct provies direct proxy support for proxy.Client
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

type Direct struct {
	proxy.ProxyCommonStruct
	*netLayer.UDP_Pipe

	targetAddr *netLayer.Addr
	addrStr    string
}

type ClientCreator struct{}

func NewClient() (proxy.Client, error) {
	d := &Direct{
		UDP_Pipe: netLayer.NewUDP_Pipe(),
	}
	go netLayer.RelayUDP_to_Direct(d.UDP_Pipe)
	return d, nil
}

func (_ ClientCreator) NewClientFromURL(*url.URL) (proxy.Client, error) {
	return NewClient()
}

func (_ ClientCreator) NewClient(*proxy.DialConf) (proxy.Client, error) {
	return NewClient()
}

func (d *Direct) Name() string { return name }

func (d *Direct) Handshake(underlay net.Conn, target *netLayer.Addr) (io.ReadWriter, error) {

	if underlay == nil {
		d.targetAddr = target
		d.SetAddrStr(d.targetAddr.String())
		return target.Dial()
	}

	return underlay, nil

}
