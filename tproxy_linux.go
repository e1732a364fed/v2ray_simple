package v2ray_simple

import (
	"go.uber.org/zap"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//非阻塞。监听透明代理, 返回一个 值 用于 关闭.
func ListenTproxy(addr string, defaultOutClientForThis proxy.Client, routePolicy *netLayer.RoutePolicy) (tm *tproxy.Machine) {
	utils.Info("Start running Tproxy")

	ad, err := netLayer.NewAddr(addr)
	if err != nil {
		panic(err)
	}
	//因为 tproxy比较特殊, 不属于 proxy.Server, 所以 需要 独立的 转发过程去处理.
	lis, err := startLoopTCP(ad, defaultOutClientForThis, &proxy.RoutingEnv{
		RoutePolicy: routePolicy,
	})
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopTCP failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	udpConn := startLoopUDP(ad, defaultOutClientForThis, &proxy.RoutingEnv{
		RoutePolicy: routePolicy,
	})

	tm = &tproxy.Machine{Addr: ad, Listener: lis, UDPConn: udpConn}

	return
}

//非阻塞
func startLoopTCP(ad netLayer.Addr, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) (net.Listener, error) {
	return netLayer.ListenAndAccept("tcp", ad.String(), &netLayer.Sockopt{TProxy: true}, 0, func(conn net.Conn) {
		tcpconn := conn.(*net.TCPConn)
		targetAddr := tproxy.HandshakeTCP(tcpconn)

		if ce := utils.CanLogInfo("TProxy loop read got new tcp"); ce != nil {
			ce.Write(zap.String("->", targetAddr.String()))
		}

		passToOutClient(incomingInserverConnState{
			wrappedConn:   tcpconn,
			defaultClient: defaultOutClientForThis,
			RoutingEnv:    env,
		}, false, tcpconn, nil, targetAddr)
	})

}

//非阻塞
func startLoopUDP(ad netLayer.Addr, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) *net.UDPConn {
	ad.Network = "udp"
	conn, err := ad.ListenUDP_withOpt(&netLayer.Sockopt{TProxy: true})
	if err != nil {
		if ce := utils.CanLogErr("TProxy startLoopUDP DialWithOpt failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}
	udpConn := conn.(*net.UDPConn)
	go func() {

		for {
			msgConn, raddr, err := tproxy.HandshakeUDP(udpConn)
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

			go passToOutClient(incomingInserverConnState{
				defaultClient: defaultOutClientForThis,
				RoutingEnv:    env,
			}, false, nil, msgConn, raddr)
		}

	}()
	return udpConn
}
