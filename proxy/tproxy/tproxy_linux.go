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

const name = "tproxy"

func init() {
	proxy.RegisterServer(name, &ServerCreator{})
}

type ServerCreator struct{ proxy.CreatorCommonStruct }

func (ServerCreator) URLToListenConf(url *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {
	if format != proxy.StandardMode {
		return lc, utils.ErrUnImplemented
	}
	if lc == nil {
		return nil, utils.ErrNilParameter
	}

	return lc, nil
}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {

	s := &Server{}
	if thing := lc.Extra["auto_iptables"]; thing != nil {
		if auto, ok := utils.AnyToBool(thing); ok && auto {
			s.shouldSetIPTable = true
		}
	}
	return s, nil
}

func (ServerCreator) AfterCommonConfServer(ps proxy.Server) (err error) {
	s := ps.(*Server)
	if s.shouldSetIPTable {
		err = tproxy.SetIPTablesByPort(s.ListenConf.Port)
	}
	return
}

// implements proxy.ListenerServer
type Server struct {
	proxy.Base

	shouldSetIPTable bool

	infoChan    chan<- proxy.IncomeTCPInfo
	udpInfoChan chan<- proxy.IncomeUDPInfo
	tm          *tproxy.Machine
	sync.Once
}

func NewServer() (proxy.Server, error) {
	d := &Server{}
	return d, nil
}
func (*Server) Name() string { return name }

func (*Server) SelfListen() (is, tcp, udp bool) {
	return true, true, true
}

func (s *Server) Close() error {
	s.Stop()
	return nil
}

func (s *Server) Stop() {
	s.Once.Do(func() {
		if s.infoChan != nil {
			close(s.infoChan)
		}
		if s.udpInfoChan != nil {
			close(s.udpInfoChan)
		}

		s.tm.Close()
	})

}

func (s *Server) StartListen(infoChan chan<- proxy.IncomeTCPInfo, udpInfoChan chan<- proxy.IncomeUDPInfo) io.Closer {
	s.infoChan = infoChan
	s.udpInfoChan = udpInfoChan

	lis, err := netLayer.ListenAndAccept("tcp", s.Addr, &netLayer.Sockopt{TProxy: true}, 0, func(conn net.Conn) {
		tcpconn := conn.(*net.TCPConn)
		targetAddr := tproxy.HandshakeTCP(tcpconn)

		info := proxy.IncomeTCPInfo{
			Conn:   tcpconn,
			Target: targetAddr,
		}

		if ce := utils.CanLogInfo("TProxy loop read got new tcp"); ce != nil {
			ce.Write(zap.String("->", targetAddr.String()))
		}
		infoChan <- info
	})
	if err != nil {
		if ce := utils.CanLogErr("TProxy listen tcp failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
	}

	ad, err := netLayer.NewAddr(s.Addr)
	if err != nil {
		if ce := utils.CanLogErr("TProxy convert listen addr failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
	}

	uconn, err := ad.ListenUDP_withOpt(&netLayer.Sockopt{TProxy: true})
	if err != nil {
		if ce := utils.CanLogErr("TProxy listen udp failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

	udpConn := uconn.(*net.UDPConn)

	tm := &tproxy.Machine{Addr: ad, Listener: lis, UDPConn: udpConn}
	tm.Init()

	go func() {
		for {
			msgConn, raddr, err := tm.HandshakeUDP(udpConn)
			if err != nil {
				if ce := utils.CanLogErr("TProxy startLoopUDP loop read failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
				break
			} else {
				if ce := utils.CanLogInfo("TProxy loop read got new udp"); ce != nil {
					ce.Write(zap.String("->", raddr.String()))
				}
			}
			msgConn.SetFullcone(s.IsFullcone)

			udpInfoChan <- proxy.IncomeUDPInfo{MsgConn: msgConn, Target: raddr}

		}
	}()
	s.tm = tm

	return s
}

func (s *Server) Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error) {
	return nil, nil, netLayer.Addr{}, utils.ErrUnImplemented
}
