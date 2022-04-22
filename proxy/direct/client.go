// Package direct provies a struct that implements proxy.Client
package direct

import (
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

const Name = "direct"

func init() {
	proxy.RegisterClient(Name, &ClientCreator{})
}

type ClientCreator struct{}

func (ClientCreator) NewClientFromURL(url *url.URL) (proxy.Client, error) {
	d := &Client{}

	nStr := url.Query().Get("fullcone")
	if nStr == "true" || nStr == "1" {
		d.isfullcone = true
	}

	return d, nil
}

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {
	d := &Client{}
	d.isfullcone = dc.Fullcone
	return d, nil
}

//实现了 proxy.Client
type Client struct {
	proxy.ProxyCommonStruct
	isfullcone bool
}

func (*Client) Name() string { return Name }

//若 underlay 为nil，则我们会自动对target进行拨号, 否则直接返回underlay。
func (d *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (result io.ReadWriteCloser, err error) {

	if underlay == nil {
		result, err = target.Dial()

	} else {
		result = underlay

	}
	if err != nil {
		return
	}
	if len(firstPayload) > 0 {
		_, err = underlay.Write(firstPayload)
		utils.PutBytes(firstPayload)

	}

	return

}

//direct的Client的 EstablishUDPChannel 实际上就是直接 监听一个udp端口。会无视传入的net.Conn.
func (d *Client) EstablishUDPChannel(_ net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {

	return netLayer.NewUDPMsgConn(nil, d.isfullcone, false), nil
}
