package tun

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tun"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

const name = "tun"

type ServerCreator struct{ proxy.CreatorCommonStruct }

func (ServerCreator) URLToListenConf(url *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {
	if lc == nil {
		return nil, utils.ErrNilParameter
	}

	return lc, nil
}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {

	s := &Server{}

	return s, nil
}

type Server struct {
	proxy.Base

	stopped bool

	infoChan chan<- netLayer.TCPRequestInfo
}

func (*Server) Name() string { return name }

func (s *Server) SelfListen() (is, tcp, udp bool) {
	is = true
	tcp = true
	udp = true
	return
}

func (s *Server) Close() error {
	s.Stop()
	return nil
}

func (s *Server) Stop() {
	if !s.stopped {
		s.stopped = true
		close(s.infoChan)
	}

}

func (s *Server) StartListen(infoChan chan<- netLayer.TCPRequestInfo, udpInfoChan chan<- netLayer.UDPRequestInfo) io.Closer {
	tunDev, err := tun.ListenTun()
	if err != nil {
		if ce := utils.CanLogErr("tun listen failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}
	s.infoChan = infoChan

	tchan, _ := tun.HandleTun(tunDev)
	go func() {
		for tr := range tchan {
			if s.stopped {
				return
			}
			infoChan <- tr

		}
	}()

	return s
}

func (s *Server) Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error) {
	return nil, nil, netLayer.Addr{}, utils.ErrUnImplemented
}
