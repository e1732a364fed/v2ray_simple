package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const DirectName = "direct"

//implements ClientCreator for direct
type DirectCreator struct{}

func (DirectCreator) NewClientFromURL(url *url.URL) (Client, error) {
	d := &DirectClient{}

	nStr := url.Query().Get("fullcone")
	if nStr == "true" || nStr == "1" {
		d.isfullcone = true
	}

	return d, nil
}

func (DirectCreator) NewClient(dc *DialConf) (Client, error) {
	d := &DirectClient{}
	d.isfullcone = dc.Fullcone
	return d, nil
}

type DirectClient struct {
	Base
	isfullcone bool
}

func (*DirectClient) Name() string { return DirectName }

//若 underlay 为nil，则会对target进行拨号, 否则返回underlay本身
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

//direct的Client的 EstablishUDPChannel 直接 监听一个udp端口，无视传入的net.Conn.
func (d *DirectClient) EstablishUDPChannel(_ net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {
	return netLayer.NewUDPMsgConn(nil, d.isfullcone, false)
}
