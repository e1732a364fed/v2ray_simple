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

func init() {
	proxy.RegisterServer(name, &ServerCreator{})
}

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

	infoChan       chan<- netLayer.TCPRequestInfo
	udpRequestChan chan<- netLayer.UDPRequestInfo
	lwipCloser     io.Closer
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
		s.lwipCloser.Close()
		close(s.infoChan)
		close(s.udpRequestChan)
	}

}

func (s *Server) StartListen(tcpRequestChan chan<- netLayer.TCPRequestInfo, udpRequestChan chan<- netLayer.UDPRequestInfo) io.Closer {
	tunDev, err := tun.CreateTun("", "10.1.0.10", "10.1.0.20", "255.255.255.0")
	if err != nil {
		if ce := utils.CanLogErr("tun listen failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}
	s.infoChan = tcpRequestChan
	s.udpRequestChan = udpRequestChan

	newTchan, newUchan, lwipcloser := tun.ListenTun(tunDev)
	go func() {
		for tr := range newTchan {
			if s.stopped {
				return
			}
			if ce := utils.CanLogInfo("tun got new tcp"); ce != nil {
				ce.Write(zap.String("->", tr.Target.String()))
			}
			tcpRequestChan <- tr

		}
	}()

	go func() {
		for ur := range newUchan {
			if s.stopped {
				return
			}
			if ce := utils.CanLogInfo("tun got new udp"); ce != nil {
				ce.Write(zap.String("->", ur.Target.String()))
			}
			udpRequestChan <- ur
		}
	}()
	s.lwipCloser = lwipcloser

	return s
}

func (s *Server) Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error) {
	return nil, nil, netLayer.Addr{}, utils.ErrUnImplemented
}
