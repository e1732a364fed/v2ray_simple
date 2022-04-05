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

//实现了 proxy.Client, netLayer.UDP_Extractor, netLayer.UDP_Putter_Generator
type Direct struct {
	proxy.ProxyCommonStruct
}

type ClientCreator struct{}

func NewClient() (proxy.Client, error) {
	d := &Direct{}
	return d, nil
}

func (_ ClientCreator) NewClientFromURL(*url.URL) (proxy.Client, error) {
	return NewClient()
}

func (_ ClientCreator) NewClient(*proxy.DialConf) (proxy.Client, error) {
	return NewClient()
}

func (d *Direct) Name() string { return name }

//若 underlay 为nil，则我们会自动对target进行拨号。
func (d *Direct) Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error) {

	if underlay == nil {
		return target.Dial()
	}

	return underlay, nil

}

func (d *Direct) GetNewUDP_Putter() netLayer.UDP_Putter {

	//单单的pipe是无法做到转发的，它就像一个缓存一样;
	// 一方是未知的,向 UDP_Putter 放入请求,
	// 然后我们这边就要通过一个 goroutine 来不断提取请求然后转发到direct.

	pipe := netLayer.NewUDP_Pipe()
	go netLayer.RelayUDP_to_Direct(pipe)
	return pipe
}
