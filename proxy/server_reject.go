package proxy

import (
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func (RejectCreator) NewServerFromURL(url *url.URL) (Server, error) {
	r := &RejectServer{}
	nStr := url.Query().Get("type")
	if nStr != "" {
		r.theType = nStr
	}

	return r, nil
}

func (RejectCreator) NewServer(dc *ListenConf) (Server, error) {
	r := &RejectServer{}

	if dc.Extra != nil {
		if thing := dc.Extra["type"]; thing != nil {
			if t, ok := thing.(string); ok && t != "" {
				r.theType = t
			}
		}
	}

	return r, nil
}

//mimic the behavior of RejectClient
type RejectServer struct {
	Base

	theType string
}

func (*RejectServer) Name() string { return RejectName }

//return utils.ErrHandled
func (s *RejectServer) Handshake(underlay net.Conn) (_ net.Conn, _ netLayer.MsgConn, _ netLayer.Addr, e error) {
	tryRejectWithHttpRespAndClose(s.theType, underlay)

	e = utils.ErrHandled
	return
}
