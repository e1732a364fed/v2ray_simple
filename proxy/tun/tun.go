// Package tun implements proxy.Server for tun device.
/*
	tun Server使用 host 配置作为 tun device name
	使用 ip 配置作为 gateway 的ip
	使用 extra.tun_selfip 作为 tun向外拨号的ip

	tun device name的默认值约定： mac utun5, windows vs_wintun

*/
package tun

import (
	"io"
	"net"
	"net/url"
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tun"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

const (
	name         = "tun"
	manualPrompt = "Please try run these commands manually(Administrator):"
)

var (
	AddManualRunCmdsListFunc func([]string)
	rememberedRouterIP       string
	manualRoute              bool
	autoRoutePreFunc         func(tunDevName, tunGateway, tunIP string, directlist []string) bool
	autoRouteFunc            func(tunDevName, tunGateway, tunIP string, directlist []string)
	autoRouteDownFunc        func(tunDevName, tunGateway, tunIP string, directlist []string)
)

func promptManual(strs []string) {
	utils.Warn(manualPrompt)
	for _, s := range strs {
		utils.Warn(s)
	}

	if AddManualRunCmdsListFunc != nil {
		AddManualRunCmdsListFunc(strs)
	}
}

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
	s.devName = lc.Host
	s.realIP = lc.IP
	if len(lc.Extra) > 0 {
		if thing := lc.Extra["tun_selfip"]; thing != nil {
			if str, ok := thing.(string); ok {
				s.selfip = str
			}
		}

		if thing := lc.Extra["tun_auto_route"]; thing != nil {
			if auto, autoOk := utils.AnyToBool(thing); autoOk && auto {

				if thing := lc.Extra["tun_auto_route_direct_list"]; thing != nil {

					if list, ok := thing.([]any); ok {
						for _, v := range list {
							if str, ok := v.(string); ok && str != "" {
								s.autoRouteDirectList = append(s.autoRouteDirectList, str)
							}
						}
					}
				}

				if len(s.autoRouteDirectList) == 0 {
					utils.Warn("tun auto route set, but no direct list given. auto route will not run.")
				} else {
					s.autoRoute = true

				}

				if thing := lc.Extra["tun_auto_route_manual"]; thing != nil {

					if manual, ok := utils.AnyToBool(thing); ok && manual {
						manualRoute = true
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
	//const defaultMask = "255.255.255.0"

	//上面两个默认ip取自water项目给出的示例

	if s.realIP == "" {
		s.realIP = defaultRealIP
	}
	if s.selfip == "" {
		s.selfip = defaultSelfIP
	}

	return
}

type Server struct {
	proxy.Base

	stopped bool

	infoChan       chan<- netLayer.TCPRequestInfo
	udpRequestChan chan<- netLayer.UDPRequestInfo
	lwipCloser     io.Closer

	devName, realIP, selfip string //selfip 只在 darwin 上用到
	autoRoute               bool
	autoRouteDirectList     []string
}

func (*Server) Name() string { return name }

func (s *Server) SelfListen() (is bool, tcp, udp int) {
	switch n := s.Network(); n {
	case "", netLayer.DualNetworkName:
		tcp = 1
		udp = 1

	case "tcp":
		tcp = 1
		udp = -1
	case "udp":
		udp = 1
		tcp = -1
	}

	is = true

	return
}

func (s *Server) Close() error {
	s.Stop()
	return nil
}

func (s *Server) Stop() {
	if !s.stopped {
		s.stopped = true

		if s.autoRoute && autoRouteDownFunc != nil {
			utils.Info("tun running auto table down")

			autoRouteDownFunc(s.devName, s.realIP, s.selfip, s.autoRouteDirectList)
		}

		close(s.infoChan)
		close(s.udpRequestChan)
		s.lwipCloser.Close()
	}

}

func (s *Server) StartListen(tcpRequestChan chan<- netLayer.TCPRequestInfo, udpRequestChan chan<- netLayer.UDPRequestInfo) io.Closer {
	s.stopped = false

	if s.devName == "" {
		switch runtime.GOOS {
		case "darwin":
			s.devName = "utun5"
		case "windows":
			s.devName = "vs_wintun"
		case "liunx":
			s.devName = "vs_tun"
		}
	}

	//由于我们目前完全使用 xjasonlyu/tun2socks 的 代码，需要注意，xjasonlyu/tun2socks 在
	// windows 和 darwin 都是用的是 wiregard包来创建 tun, 而 在linux上是采用的 gvisor包创建的
	// 至于为什么这么做，本作作者暂时不清楚。

	// 不过这就导致了 linux 和其他系统的一点不同，那就是，在linux上我们要先手动用命令创建tun，然后再 运行本程序,
	// 可以参考 https://github.com/xjasonlyu/tun2socks/wiki/Examples

	//所以这里如果开启了 auto_route, 我们应该自动帮用户提前创建 tun, 减轻用户使用负担

	if s.autoRoute && autoRoutePreFunc != nil {
		autoRoutePreFunc(s.devName, s.realIP, s.selfip, s.autoRouteDirectList)

	}

	tunDev, err := tun.Open(s.devName)
	if err != nil {
		if ce := utils.CanLogErr("tun open failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		s.stopped = true
		return nil
	}

	if s.autoRoute && autoRouteFunc != nil {
		utils.Info("tun running auto table")
		autoRouteFunc(s.devName, s.realIP, s.selfip, s.autoRouteDirectList)
	}
	s.infoChan = tcpRequestChan
	s.udpRequestChan = udpRequestChan

	newTchan, newUchan, closer, err := tun.Listen(tunDev)

	if err != nil {
		if ce := utils.CanLogErr("tun listen failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

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
	s.lwipCloser = closer

	return s
}

func (s *Server) Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error) {
	return nil, nil, netLayer.Addr{}, utils.ErrUnImplemented
}
