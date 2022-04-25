package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const directName = "direct"

func init() {
	RegisterClient(directName, &DirectClientCreator{})
}

type DirectClientCreator struct{}

func (DirectClientCreator) NewClientFromURL(url *url.URL) (Client, error) {
	d := &DirectClient{}

	nStr := url.Query().Get("fullcone")
	if nStr == "true" || nStr == "1" {
		d.isfullcone = true
	}

	return d, nil
}

func (DirectClientCreator) NewClient(dc *DialConf) (Client, error) {
	d := &DirectClient{}
	d.isfullcone = dc.Fullcone
	return d, nil
}

//实现了 DirectClient
type DirectClient struct {
	ProxyCommonStruct
	isfullcone bool
}

func (*DirectClient) Name() string { return directName }

//若 underlay 为nil，则我们会自动对target进行拨号, 否则直接返回underlay。
func (d *DirectClient) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (result io.ReadWriteCloser, err error) {

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
func (d *DirectClient) EstablishUDPChannel(_ net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {
	return netLayer.NewUDPMsgConn(nil, d.isfullcone, false)
}
