// Package tproxy implements proxy.Server for tproxy.
package tproxy

import (
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

//在darwin上，代码相当于linux上的精简版本, 不提供udp支持

const name = "tproxy"

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
	if thing := lc.Extra["auto_iptables"]; thing != nil {
		if auto, ok := utils.AnyToBool(thing); ok && auto {
			s.shouldSetRoute = true
		}
	}
	return s, nil
}

func (ServerCreator) AfterCommonConfServer(ps proxy.Server) (err error) {
	s := ps.(*Server)

	if s.shouldSetRoute {
		err = tproxy.SetRouteByPort(s.ListenConf.Port)
	}
	return
}

// implements proxy.ListenerServer
type Server struct {
	proxy.Base

	shouldSetRoute bool

	infoChan chan<- netLayer.TCPRequestInfo
	tm       *tproxy.Machine
	sync.Once
}

func NewServer() (proxy.Server, error) {
	d := &Server{}
	return d, nil
}
func (*Server) Name() string { return name }

func (s *Server) SelfListen() (is, tcp, udp bool) {
	return true, true, true //darwin的tproxy不支持udp，但是如果传入false，则v2ray_simple包会试图自行监听udp。
	//所以暂时的解决方案是欺骗它告诉他我们都监听
}

func (s *Server) Close() error {
	s.Stop()
	return nil
}

func (s *Server) Stop() {
	s.Once.Do(func() {
		s.tm.Stop()

		if s.infoChan != nil {
			close(s.infoChan)
		}

	})

}

func (s *Server) StartListen(infoChan chan<- netLayer.TCPRequestInfo, udpInfoChan chan<- netLayer.UDPRequestInfo) io.Closer {

	tm := new(tproxy.Machine)
	_, lt, _ := s.SelfListen()

	if lt {
		s.infoChan = infoChan

		lis, err := netLayer.ListenAndAccept("tcp", s.Addr, s.Sockopt, 0, func(conn net.Conn) {
			tcpconn := conn.(*net.TCPConn)
			ta, err := tproxy.HandshakeTCP(tcpconn)
			if err != nil {
				if ce := utils.CanLogErr("tproxy HandshakeTCP failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
				return
			}
			targetAddr := netLayer.NewAddrFromTCPAddr(ta)

			info := netLayer.TCPRequestInfo{
				Conn:   tcpconn,
				Target: targetAddr,
			}

			if ce := utils.CanLogInfo("TProxy got new tcp"); ce != nil {
				ce.Write(zap.String("->", targetAddr.String()))
			}
			if tm.Closed() {
				return
			}
			infoChan <- info
		})
		if err != nil {
			if ce := utils.CanLogErr("TProxy listen tcp failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}
		tm.Listener = lis

	}

	tm.Init()
	s.tm = tm

	return s
}

func (s *Server) Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error) {
	return nil, nil, netLayer.Addr{}, utils.ErrUnImplemented
}
