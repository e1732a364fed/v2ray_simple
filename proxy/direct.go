package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	DirectName = "direct"
	DirectURL  = DirectName + "://"
)

//implements ClientCreator for direct
type DirectCreator struct{}

func (DirectCreator) NewClientFromURL(url *url.URL) (Client, error) {
	d := &DirectClient{}

	return d, nil
}

func (DirectCreator) NewClient(dc *DialConf) (Client, error) {
	d := &DirectClient{}
	if dc.Sockopt != nil {
		opt := *dc.Sockopt
		d.Sockopt = &opt
	}
	return d, nil
}

type DirectClient struct {
	Base
}

func (*DirectClient) Name() string { return DirectName }

//若 underlay 为nil，则会对target进行拨号, 否则返回underlay本身
func (d *DirectClient) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (result io.ReadWriteCloser, err error) {

	if underlay == nil {
		if d.Sockopt != nil {
			result, err = target.DialWithOpt(d.Sockopt)
		} else {
			result, err = target.Dial()
		}

	} else {
		result = underlay

	}
	if err != nil {
		return
	}
	if len(firstPayload) > 0 {
		_, err = result.Write(firstPayload)
		utils.PutBytes(firstPayload)

	}

	return

}

//direct的Client的 EstablishUDPChannel 直接 监听一个udp端口，无视传入的net.Conn.
func (d *DirectClient) EstablishUDPChannel(_ net.Conn, firstPayload []byte, target netLayer.Addr) (netLayer.MsgConn, error) {
	if len(firstPayload) == 0 {
		return netLayer.NewUDPMsgConn(nil, d.IsFullcone, false, d.Sockopt)

	} else {
		mc, err := netLayer.NewUDPMsgConn(nil, d.IsFullcone, false, d.Sockopt)
		if err != nil {
			return nil, err
		}
		return mc, mc.WriteMsgTo(firstPayload, target)

	}
}
