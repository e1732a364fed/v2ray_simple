// Package tun implements proxy.Server for tun device.
/*
	tun Server使用 host 配置作为 tun device name
	使用 ip 配置作为 gateway 的ip
	使用 extra.tun_selfip 作为 tun向外拨号的ip
	使用 extra.tun_mask 作为 子网掩码
*/
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

var (
	autoRouteFunc     func(tunDevName, tunGateway, tunIP string, directlist []string)
	autoRouteDownFunc func(tunDevName, tunGateway, tunIP string, directlist []string)
)

func (ServerCreator) URLToListenConf(url *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {
	if lc == nil {
		return nil, utils.ErrNilParameter
	}

	return lc, nil
}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {

	s := &Server{}
	s.devName = lc.Host
	s.realIP = lc.IP
	if len(lc.Extra) > 0 {
		if thing := lc.Extra["tun_selfip"]; thing != nil {
			if str, ok := thing.(string); ok {
				s.selfip = str
			}
		}
		if thing := lc.Extra["tun_mask"]; thing != nil {
			if str, ok := thing.(string); ok {
				s.mask = str
			}
		}

		if thing := lc.Extra["tun_auto_route"]; thing != nil {
			if auto, autoOk := utils.AnyToBool(thing); autoOk && auto {
				s.autoRoute = true

				if thing := lc.Extra["tun_auto_route_direct_list"]; thing != nil {

					if list, ok := thing.([]any); ok {
						for _, v := range list {
							if str, ok := v.(string); ok && str != "" {
								s.autoRouteDirectList = append(s.autoRouteDirectList, str)
							}
						}
					}
				}

			}
		}
	}

	return s, nil
}

func (ServerCreator) AfterCommonConfServer(ps proxy.Server) (err error) {
	s := ps.(*Server)

	const defaultSelfIP = "10.1.0.10"
	const defaultRealIP = "10.1.0.20"
	const defaultMask = "255.255.255.0"

	//上面两个默认ip取自water项目给出的示例

	if s.realIP == "" {
		s.realIP = defaultRealIP
	}
	if s.selfip == "" {
		s.selfip = defaultSelfIP
	}
	if s.mask == "" {
		s.mask = defaultMask
	}

	return
}

type Server struct {
	proxy.Base

	stopped bool

	infoChan       chan<- netLayer.TCPRequestInfo
	udpRequestChan chan<- netLayer.UDPRequestInfo
	lwipCloser     io.Closer

	devName, realIP, selfip, mask string
	autoRoute                     bool
	autoRouteDirectList           []string
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
		if s.autoRoute && autoRouteDownFunc != nil {
			utils.Info("tun running auto table down")

			autoRouteDownFunc(s.devName, s.realIP, s.selfip, s.autoRouteDirectList)
		}
	}

}

func (s *Server) StartListen(tcpRequestChan chan<- netLayer.TCPRequestInfo, udpRequestChan chan<- netLayer.UDPRequestInfo) io.Closer {
	//log.Println(s.devName, s.selfip, s.realIP, s.mask)
	realname, tunDev, err := tun.CreateTun(s.devName, s.selfip, s.realIP, s.mask)
	if err != nil {
		if ce := utils.CanLogErr("tun listen failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}
	s.devName = realname
	if s.autoRoute && autoRouteFunc != nil {
		utils.Info("tun running auto table")
		autoRouteFunc(realname, s.realIP, s.selfip, s.autoRouteDirectList)
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
