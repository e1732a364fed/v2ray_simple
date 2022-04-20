package main

import (
	"go.uber.org/zap"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer/tproxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func listenTproxy(addr string) {
	utils.Info("Start running Tproxy ")

	ad, err := netLayer.NewAddr(addr)
	if err != nil {
		panic(err)
	}
	var tp TProxy = TProxy(ad)
	go tp.StartLoopTCP()
	go tp.StartLoopUDP()

	tproxyListenCount++
}

var tproxyList []TProxy

//tproxy因为比较特殊, 不属于 proxy.Server, 需要独特的转发过程去处理.
type TProxy netLayer.Addr

func (tp TProxy) StartLoop() {
	tp.StartLoopTCP()
}

func (tp TProxy) StartLoopTCP() {
	ad := netLayer.Addr(tp)
	netLayer.ListenAndAccept("tcp", ad.String(), &netLayer.Sockopt{TProxy: true}, func(conn net.Conn) {
		tcpconn := conn.(*net.TCPConn)
		targetAddr := tproxy.HandshakeTCP(tcpconn)

		if ce := utils.CanLogErr("TProxy loop read got new tcp"); ce != nil {
			ce.Write(zap.String("->", targetAddr.String()))
		}

		passToOutClient(incomingInserverConnState{
			wrappedConn: tcpconn,
		}, false, tcpconn, nil, targetAddr)
	})

}

func (tp TProxy) StartLoopUDP() {
	ad := netLayer.Addr(tp)
	ad.Network = "udp"
	conn, err := ad.DialWithOpt(&netLayer.Sockopt{TProxy: true})
	if err != nil {
		if ce := utils.CanLogErr("TProxy StartLoopUDP DialWithOpt failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	udpConn := conn.(*net.UDPConn)
	for {
		msgConn, raddr, err := tproxy.HandshakeUDP(udpConn)
		if err != nil {
			if ce := utils.CanLogErr("TProxy StartLoopUDP loop read failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			break
		} else {
			if ce := utils.CanLogErr("TProxy loop read got new udp"); ce != nil {
				ce.Write(zap.String("->", raddr.String()))
			}
		}

		go passToOutClient(incomingInserverConnState{}, false, nil, msgConn, raddr)
	}
}
