package v2ray_simple

import (
	"go.uber.org/zap"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//非阻塞。监听透明代理
func ListenTproxy(lc proxy.LesserConf, defaultOutClientForThis proxy.Client, routePolicy *netLayer.RoutePolicy) (tm *tproxy.Machine) {
	utils.Info("Start running Tproxy")

	ad, err := netLayer.NewAddr(lc.Addr)
	if err != nil {
		panic(err)
	}
	//因为 tproxy比较特殊, 不属于 proxy.Server, 所以 需要 独立的 转发过程去处理.
	lis, err := startLoopTCP(ad, lc, defaultOutClientForThis, &proxy.RoutingEnv{
		RoutePolicy: routePolicy,
	})
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopTCP failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	ad.Network = "udp"
	uconn, err := ad.ListenUDP_withOpt(&netLayer.Sockopt{TProxy: true})
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopUDP DialWithOpt failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

	udpConn := uconn.(*net.UDPConn)

	tm = &tproxy.Machine{Addr: ad, Listener: lis, UDPConn: udpConn}
	tm.Init()

	go startLoopUDP(udpConn, tm, lc, defaultOutClientForThis, &proxy.RoutingEnv{
		RoutePolicy: routePolicy,
	})

	return
}

//非阻塞
func startLoopTCP(ad netLayer.Addr, lc proxy.LesserConf, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) (net.Listener, error) {
	return netLayer.ListenAndAccept("tcp", ad.String(), &netLayer.Sockopt{TProxy: true}, 0, func(conn net.Conn) {
		tcpconn := conn.(*net.TCPConn)
		targetAddr := tproxy.HandshakeTCP(tcpconn)

		if ce := utils.CanLogInfo("TProxy loop read got new tcp"); ce != nil {
			ce.Write(zap.String("->", targetAddr.String()))
		}

		passToOutClient(incomingInserverConnState{
			inTag:         lc.Tag,
			useSniffing:   lc.UseSniffing,
			wrappedConn:   tcpconn,
			defaultClient: defaultOutClientForThis,
			routingEnv:    env,
		}, false, tcpconn, nil, targetAddr)
	})

}

//阻塞
func startLoopUDP(udpConn *net.UDPConn, tm *tproxy.Machine, lc proxy.LesserConf, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) {

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
		msgConn.SetFullcone(lc.Fullcone)

		go passToOutClient(incomingInserverConnState{
			inTag:         lc.Tag,
			useSniffing:   lc.UseSniffing,
			defaultClient: defaultOutClientForThis,
			routingEnv:    env,
		}, false, nil, msgConn, raddr)
	}

}
