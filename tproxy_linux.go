package v2ray_simple

// //非阻塞。监听透明代理
// func ListenTproxy(lc proxy.LesserConf, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) (tm *tproxy.Machine) {
// 	utils.Info("Start running Tproxy")

// 	ad, err := netLayer.NewAddr(lc.Addr)
// 	if err != nil {
// 		panic(err)
// 	}
// 	//因为 tproxy比较特殊, 不属于 proxy.Server, 所以 需要 独立的 转发过程去处理.
// 	lis, err := startLoopTCP(ad, lc, defaultOutClientForThis, env)
// 	if err != nil {
// 		if ce := utils.CanLogErr("TProxy startLoopTCP failed"); ce != nil {
// 			ce.Write(zap.Error(err))
// 		}
// 		return
// 	}

// 	ad.Network = "udp"
// 	uconn, err := ad.ListenUDP_withOpt(&netLayer.Sockopt{TProxy: true})
// 	if err != nil {
// 		if ce := utils.CanLogErr("TProxy startLoopUDP DialWithOpt failed"); ce != nil {
// 			ce.Write(zap.Error(err))
// 		}
// 		return
// 	}

// 	udpConn := uconn.(*net.UDPConn)

// 	tm = &tproxy.Machine{Addr: ad, Listener: lis, UDPConn: udpConn}
// 	tm.Init()

// 	go startLoopUDP(udpConn, tm, lc, defaultOutClientForThis, env)

// 	return
// }

// //非阻塞
// func startLoopTCP(ad netLayer.Addr, lc proxy.LesserConf, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) (net.Listener, error) {
// 	return netLayer.ListenAndAccept("tcp", ad.String(), &netLayer.Sockopt{TProxy: true}, 0, func(conn net.Conn) {
// 		tcpconn := conn.(*net.TCPConn)
// 		targetAddr := tproxy.HandshakeTCP(tcpconn)

// 		if ce := utils.CanLogInfo("TProxy loop read got new tcp"); ce != nil {
// 			ce.Write(zap.String("->", targetAddr.String()))
// 		}

// 		passToOutClient(incomingInserverConnState{
// 			inTag:         lc.Tag,
// 			useSniffing:   lc.UseSniffing,
// 			wrappedConn:   tcpconn,
// 			defaultClient: defaultOutClientForThis,
// 			routingEnv:    env,
// 		}, false, tcpconn, nil, targetAddr)
// 	})

// }

// //阻塞
// func startLoopUDP(udpConn *net.UDPConn, tm *tproxy.Machine, lc proxy.LesserConf, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) {

// 	for {
// 		msgConn, raddr, err := tm.HandshakeUDP(udpConn)
// 		if err != nil {
// 			if ce := utils.CanLogErr("TProxy startLoopUDP loop read failed"); ce != nil {
// 				ce.Write(zap.Error(err))
// 			}
// 			break
// 		} else {
// 			if ce := utils.CanLogInfo("TProxy loop read got new udp"); ce != nil {
// 				ce.Write(zap.String("->", raddr.String()))
// 			}
// 		}
// 		msgConn.SetFullcone(lc.Fullcone)

// 		go passToOutClient(incomingInserverConnState{
// 			inTag:         lc.Tag,
// 			useSniffing:   lc.UseSniffing,
// 			defaultClient: defaultOutClientForThis,
// 			routingEnv:    env,
// 		}, false, nil, msgConn, raddr)
// 	}

// }
