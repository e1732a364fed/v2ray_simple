package v2ray_simple

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/e1732a364fed/v2ray_simple/advLayer/grpc"
	"github.com/e1732a364fed/v2ray_simple/advLayer/quic"
	"github.com/e1732a364fed/v2ray_simple/advLayer/ws"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/dokodemo"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/http"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/simplesocks"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/socks5"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/trojan"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vless"
)

//统计数据
var (
	ActiveConnectionCount      int32
	AllDownloadBytesSinceStart uint64
	AllUploadBytesSinceStart   uint64
)

var (
	DirectClient, _, _ = proxy.ClientFromURL("direct://")

	ServersTagMap = make(map[string]proxy.Server)
	ClientsTagMap = make(map[string]proxy.Client)

	Tls_lazy_encrypt bool
	Tls_lazy_secure  bool

	//有时需要测试到单一网站的流量，此时为了避免其它干扰，可声明 一下 该域名，然后程序里会进行过滤
	//uniqueTestDomain string
)

func init() {

	flag.BoolVar(&Tls_lazy_encrypt, "lazy", false, "tls lazy encrypt (splice)")
	flag.BoolVar(&Tls_lazy_secure, "ls", false, "tls lazy secure, use special techs to ensure the tls lazy encrypt data can't be detected. Only valid at client end.")

	//flag.StringVar(&uniqueTestDomain, "td", "", "test a single domain, like www.domain.com. Only valid when loglevel=0")

}

//非阻塞. 可以 直接使用 ListenSer 函数 来手动开启新的转发流程。
// 若 env 为 nil, 则不会 进行路由或回落
func ListenSer(inServer proxy.Server, defaultOutClientForThis proxy.Client, env *proxy.RoutingEnv) (thisListener net.Listener) {

	var err error

	//quic
	if inServer.IsHandleInitialLayers() {
		//如果像quic一样自行处理传输层至tls层之间的部分，则我们跳过 handleNewIncomeConnection 函数
		// 拿到连接后直接调用 handshakeInserver_and_passToOutClient

		handleFunc := inServer.HandleInitialLayersFunc()
		if handleFunc == nil {
			panic("inServer.IsHandleInitialLayers but inServer.HandleInitialLayersFunc() returns nil")
		}

		//baseConn可以为nil，quic就是如此
		newConnChan, baseConn := handleFunc()
		if newConnChan == nil {
			utils.Error("StarthandleInitialLayers can't extablish baseConn")
			return
		}

		go func() {
			for {
				newConn, ok := <-newConnChan
				if !ok {
					utils.Error("read from SuperProxy not ok")

					quic.CloseConn(baseConn)

					return
				}

				iics := incomingInserverConnState{
					wrappedConn: newConn,
					//baseLocalConn: baseConn,	//quic是没有baseLocalConn的，因为基于udp
					// 或者说虽然有baseConn，但是并不与子连接一一对应.那个conn更类似于一个listener
					inServer:      inServer,
					defaultClient: defaultOutClientForThis,
				}

				go handshakeInserver_and_passToOutClient(iics)
			}

		}()

		if ce := utils.CanLogInfo("Listening"); ce != nil {

			ce.Write(
				zap.String("protocol", proxy.GetFullName(inServer)),
				zap.String("addr", inServer.AddrStr()),
			)
		}
		return
	}

	handleFunc := func(conn net.Conn) {
		handleNewIncomeConnection(inServer, defaultOutClientForThis, conn, env)
	}

	network := inServer.Network()
	thisListener, err = netLayer.ListenAndAccept(network, inServer.AddrStr(), inServer.GetSockopt(), handleFunc)

	if err == nil {
		if ce := utils.CanLogInfo("Listening"); ce != nil {

			ce.Write(
				zap.String("protocol", proxy.GetFullName(inServer)),
				zap.String("addr", inServer.AddrStr()),
			)
		}

	} else {
		if err != nil {
			utils.ZapLogger.Error(
				"can not listen inServer on", zap.String("addr", inServer.AddrStr()), zap.Error(err))

		}
	}
	return
}

type incomingInserverConnState struct {

	// 在多路复用的情况下, 可能产生多个 IncomingInserverConnState，
	// 共用一个 baseLocalConn, 但是 wrappedConn 各不相同。

	//这里说的多路复用指的是grpc/quic 这种包在代理层外部的;  vless内嵌 mux.cool 或者trojan内嵌smux+simplesocks 则不属于这种情况, 它们属于 innerMux

	// 要区分 多路复用的包装 是在 vless等代理的握手验证 的外部 还是 内部

	baseLocalConn net.Conn // baseLocalConn 是来自客户端的原始网络层链接
	wrappedConn   net.Conn // wrappedConn 是层层握手后,代理层握手前 包装的链接,一般为tls层或者高级层;
	inServer      proxy.Server
	defaultClient proxy.Client

	cachedRemoteAddr string
	theRequestPath   string

	inServerTlsConn            *tlsLayer.Conn
	inServerTlsRawReadRecorder *tlsLayer.Recorder

	theFallbackFirstBuffer *bytes.Buffer

	isTlsLazyServerEnd bool

	shouldCloseInSerBaseConnWhenFinish bool

	routedToDirect bool

	RoutingEnv *proxy.RoutingEnv //used in passToOutClient
}

// handleNewIncomeConnection 会处理 网络层至高级层的数据，
// 然后将代理层的处理发往 handshakeInserver_and_passToOutClient 函数。
//
// 在 listenSer 中被调用。
func handleNewIncomeConnection(inServer proxy.Server, defaultClientForThis proxy.Client, thisLocalConnectionInstance net.Conn, env *proxy.RoutingEnv) {

	iics := incomingInserverConnState{
		baseLocalConn: thisLocalConnectionInstance,
		inServer:      inServer,
		defaultClient: defaultClientForThis,
		RoutingEnv:    env,
	}

	iics.isTlsLazyServerEnd = Tls_lazy_encrypt && canLazyEncryptServer(inServer)

	wrappedConn := thisLocalConnectionInstance

	if ce := utils.CanLogInfo("New Accepted Conn"); ce != nil {

		addrstr := wrappedConn.RemoteAddr().String()
		ce.Write(
			zap.String("from", addrstr),
			zap.String("handler", proxy.GetVSI_url(inServer)),
		)

		iics.cachedRemoteAddr = addrstr
	}

	////////////////////////////// tls层 /////////////////////////////////////

	//此时，baseLocalConn里面 正常情况下, 服务端看到的是 客户端的golang的tls 拨号发出的 tls数据
	// 客户端看到的是 socks5的数据， 我们首先就是要看看socks5里的数据是不是tls，而socks5自然 IsUseTLS 是false

	// 如果是服务端的话，那就是 inServer.IsUseTLS == true, 此时，我们正常握手，然后我们需要判断的是它承载的数据

	// 每次tls试图从 原始连接 读取内容时，都会附带把原始数据写入到 这个 Recorder中

	if inServer.IsUseTLS() {

		if iics.isTlsLazyServerEnd {
			iics.inServerTlsRawReadRecorder = tlsLayer.NewRecorder()

			iics.inServerTlsRawReadRecorder.StopRecord() //先不记录，因为一开始是我们自己的tls握手包，没有意义
			teeConn := tlsLayer.NewTeeConn(wrappedConn, iics.inServerTlsRawReadRecorder)

			wrappedConn = teeConn
		}

		tlsConn, err := inServer.GetTLS_Server().Handshake(wrappedConn)
		if err != nil {

			if ce := utils.CanLogErr("tls handshake failed"); ce != nil {
				ce.Write(
					zap.String("inServer", inServer.AddrStr()),
					zap.Error(err),
				)

			}
			wrappedConn.Close()
			return
		}

		if iics.isTlsLazyServerEnd {
			//此时已经握手完毕，可以记录了
			iics.inServerTlsRawReadRecorder.StartRecord()
		}

		iics.inServerTlsConn = tlsConn
		wrappedConn = tlsConn

	}

	adv := inServer.AdvancedLayer()

	////////////////////////////// header 层 /////////////////////////////////////

	if header := inServer.HasHeader(); header != nil {

		//websocket 可以自行处理header, 不需要额外http包装
		if adv != "ws" {
			wrappedConn = &httpLayer.HeaderConn{
				Conn:        wrappedConn,
				H:           header,
				IsServerEnd: true,
			}

		}

	}

	////////////////////////////// 高级层 /////////////////////////////////////

	if adv != "" {
		switch adv {
		//quic虽然也是adv层，但是因为根本没调用 handleNewIncomeConnection 函数，所以不在此处理
		//

		case "grpc":
			//grpc不太一样, 它是多路复用的
			// 每一条建立好的 grpc 可以随时产生对新目标的请求,

			// 我们直接循环监听然后分别用 新goroutine发向 handshakeInserver_and_passToOutClient

			if ce := utils.CanLogDebug("start upgrade grpc"); ce != nil {
				ce.Write()
			}

			grpcs := inServer.GetGRPC_Server() //这个grpc server是在配置阶段初始化好的.

			grpcs.StartHandle(wrappedConn)

			//start之后，客户端就会利用同一条tcp链接 来发送多个 请求,自此就不能直接用原来的链接了;
			// 新的子请求被 grpc包 抽象成了 抽象的 conn
			//遇到chan被关闭的情况后，就会自动关闭底层连接并退出整个函数。
			for {
				newGConn, ok := <-grpcs.NewConnChan
				if !ok {
					if ce := utils.CanLogWarn("grpc getNewSubConn not ok"); ce != nil {
						ce.Write()
					}

					iics.baseLocalConn.Close()
					return
				}

				iics.wrappedConn = newGConn

				go handshakeInserver_and_passToOutClient(iics)
			}

		case "ws":

			//从ws开始就可以应用fallback了
			// 但是，不能先upgrade, 因为我们要用path分流, 所以我们要先预读;
			// 否则的话，正常的http流量频繁地被用不到的 ws 过滤器处理,会损失很多性能,而且gobwas没办法简洁地保留初始buffer.

			var rp httpLayer.RequestParser
			re := rp.ReadAndParse(wrappedConn)
			if re != nil {
				if re == httpLayer.ErrNotHTTP_Request {
					if ce := utils.CanLogErr("WS check ErrNotHTTP_Request"); ce != nil {
						ce.Write(
							zap.String("handler", inServer.AddrStr()),
						)
					}

				} else {
					if ce := utils.CanLogErr("WS check handshake read failed"); ce != nil {

						ce.Write(
							zap.String("handler", inServer.AddrStr()),
						)
					}
				}

				wrappedConn.Close()
				return
			}

			wss := inServer.GetWS_Server()

			if rp.Method != "GET" || wss.Thepath != rp.Path {
				iics.theRequestPath = rp.Path
				iics.theFallbackFirstBuffer = rp.WholeRequestBuf

				if ce := utils.CanLogDebug("WS check err"); ce != nil {

					ce.Write(
						zap.String("handler", inServer.AddrStr()),
						zap.String("reason", "path/method not match"),
						zap.String("validPath", wss.Thepath),
						zap.String("gotMethod", rp.Method),
						zap.String("gotPath", rp.Path),
					)
				}

				passToOutClient(iics, true, nil, nil, netLayer.Addr{})
				return

			}

			//此时path和method都已经匹配了, 如果还不能通过那就说明后面的header等数据不满足ws的upgrade请求格式, 肯定是非法数据了,也不用再回落
			wsConn, err := wss.Handshake(rp.WholeRequestBuf, wrappedConn)
			if err != nil {
				if ce := utils.CanLogErr("InServer ws handshake failed"); ce != nil {

					ce.Write(
						zap.String("handler", inServer.AddrStr()),
						zap.Error(err),
					)
				}

				wrappedConn.Close()
				return

			}
			wrappedConn = wsConn
		} // switch adv

	} //if adv !=""

	iics.wrappedConn = wrappedConn

	handshakeInserver_and_passToOutClient(iics)
}

//被 handshakeInserver_and_passToOutClient 调用
func handshakeInserver(iics *incomingInserverConnState) (wlc net.Conn, udp_wlc netLayer.MsgConn, targetAddr netLayer.Addr, err error) {
	inServer := iics.inServer

	wlc, udp_wlc, targetAddr, err = inServer.Handshake(iics.wrappedConn)

	if err == nil {
		if udp_wlc != nil && inServer.Name() == "socks5" {
			// socks5的 udp associate返回的是 clientFutureAddr, 而不是实际客户的第一个请求.
			//所以我们要读一次才能进行下一步。

			firstSocks5RequestData, firstSocks5RequestAddr, err2 := udp_wlc.ReadMsgFrom()
			if err2 != nil {
				if ce := utils.CanLogWarn("failed in socks5 read"); ce != nil {
					ce.Write(
						zap.String("handler", inServer.AddrStr()),
						zap.Error(err2),
					)
				}
				err = err2
				return
			}

			iics.theFallbackFirstBuffer = bytes.NewBuffer(firstSocks5RequestData)

			targetAddr = firstSocks5RequestAddr
		}

		////////////////////////////// 内层mux阶段 /////////////////////////////////////

		if muxInt, innerProxyName := inServer.HasInnerMux(); muxInt > 0 {
			if mh, ok := wlc.(proxy.MuxMarker); ok {

				innerSerConf := proxy.ListenConf{
					CommonConf: proxy.CommonConf{
						Protocol: innerProxyName,
					},
				}

				innerSer, err2 := proxy.NewServer(&innerSerConf)
				if err2 != nil {
					if ce := utils.CanLogDebug("mux inner proxy server creation failed"); ce != nil {
						ce.Write(zap.Error(err))
					}
					err = err2
					return
				}

				session := inServer.GetServerInnerMuxSession(mh)

				if session == nil {
					err = utils.ErrFailed
					return
				}

				//内层mux要对每一个子连接单独进行 子代理协议握手 以及 outClient的拨号。

				go func() {

					for {
						utils.Debug("inServer try accept smux stream ")

						stream, err := session.AcceptStream()
						if err != nil {
							if ce := utils.CanLogDebug("mux inServer try accept stream failed"); ce != nil {
								ce.Write(zap.Error(err))
							}

							session.Close()
							return
						}

						go func() {
							if ce := utils.CanLogDebug("inServer got mux stream"); ce != nil {
								ce.Write(zap.String("innerProxyName", innerProxyName))
							}

							wlc1, udp_wlc1, targetAddr1, err1 := innerSer.Handshake(stream)

							if err1 != nil {
								if ce := utils.CanLogDebug("mux inner proxy handshake failed"); ce != nil {
									ce.Write(zap.Error(err1))
								}
								newiics := *iics

								if !findoutFirstBuf(err1, &newiics) {
									return
								}
								passToOutClient(newiics, true, wlc1, udp_wlc1, targetAddr1)

							} else {

								if ce := utils.CanLogDebug("inServer mux stream handshake ok"); ce != nil {
									ce.Write(zap.String("targetAddr1", targetAddr1.String()))
								}

								passToOutClient(*iics, false, wlc1, udp_wlc1, targetAddr1)

							}

						}()

					}
				}()

				err = utils.ErrHandled
				return
			}
		}

	}

	return
}

// 在调用 passToOutClient前遇到err时调用, 若找出了buf，设置iics，并返回true
func findoutFirstBuf(err error, iics *incomingInserverConnState) bool {
	if ce := utils.CanLogWarn("failed in inServer proxy handshake"); ce != nil {
		ce.Write(
			zap.String("handler", iics.inServer.AddrStr()),
			zap.Error(err),
		)
	}

	if !iics.inServer.CanFallback() {
		iics.wrappedConn.Close()
		return false
	}

	//通过err找出 并赋值给 iics.theFallbackFirstBuffer
	{

		fe, ok := err.(*utils.ErrFirstBuffer)
		if !ok {
			// 能fallback 但是返回的 err却不是fallback err，证明遇到了更大问题，可能是底层read问题，所以也不用继续fallback了
			iics.wrappedConn.Close()
			return false
		}

		if firstbuffer := fe.First; firstbuffer == nil {
			//不应该，至少能读到1字节的。

			panic("No FirstBuffer")

		} else {
			iics.theFallbackFirstBuffer = firstbuffer

		}
	}
	return true
}

// 本函数 处理inServer的代理层数据，并在试图处理 分流和回落后，将流量导向目标，并开始Copy。
// iics 不使用指针, 因为iics不能公用，因为 在多路复用时 iics.wrappedConn 是会变化的。
//
//被 handleNewIncomeConnection 调用。
func handshakeInserver_and_passToOutClient(iics incomingInserverConnState) {

	wlc, udp_wlc, targetAddr, err := handshakeInserver(&iics)

	switch err {
	case nil:
		passToOutClient(iics, false, wlc, udp_wlc, targetAddr)

	case utils.ErrHandled:
		return

	default:
		if !findoutFirstBuf(err, &iics) {
			return
		}

		passToOutClient(iics, true, nil, nil, netLayer.Addr{})
	}

}

//查看当前配置 是否支持fallback, 并获得回落地址。
// 被 passToOutClient 调用
func checkfallback(iics incomingInserverConnState) (targetAddr netLayer.Addr, wlc net.Conn) {
	//先检查 mainFallback，如果mainFallback中各项都不满足 或者根本没有 mainFallback 再检查 defaultFallback

	if mf := iics.RoutingEnv.MainFallback; mf != nil {

		utils.Debug("Fallback check")

		var thisFallbackType byte

		theRequestPath := iics.theRequestPath

		if iics.theFallbackFirstBuffer != nil && theRequestPath == "" {
			var failreason int

			_, _, theRequestPath, failreason = httpLayer.GetRequestMethod_and_PATH_from_Bytes(iics.theFallbackFirstBuffer.Bytes(), false)

			if failreason != 0 {
				theRequestPath = ""
			}

		}

		fallback_params := make([]string, 0, 4)

		if theRequestPath != "" {
			fallback_params = append(fallback_params, theRequestPath)
			thisFallbackType |= httpLayer.Fallback_path
		}

		if inServerTlsConn := iics.inServerTlsConn; inServerTlsConn != nil {
			//默认似乎默认tls不会给出alpn和sni项？获得的是空值,也许是因为我用了自签名+insecure,所以导致server并不会设置连接好后所协商的ServerName
			// 而alpn则也是正常的, 不设置肯定就是空值
			alpn := inServerTlsConn.GetAlpn()

			if alpn != "" {
				fallback_params = append(fallback_params, alpn)
				thisFallbackType |= httpLayer.Fallback_alpn

			}

			sni := inServerTlsConn.GetSni()
			if sni != "" {
				fallback_params = append(fallback_params, sni)
				thisFallbackType |= httpLayer.Fallback_sni
			}
		}

		{
			fbAddr := mf.GetFallback(thisFallbackType, fallback_params...)

			if ce := utils.CanLogDebug("Fallback check"); ce != nil {
				if fbAddr != nil {
					ce.Write(
						zap.String("matched", fbAddr.String()),
					)
				} else {
					ce.Write(
						zap.String("no match", ""),
					)
				}
			}
			if fbAddr != nil {
				targetAddr = *fbAddr
				wlc = iics.wrappedConn
				return
			}
		}

	}

	//默认回落, 每个listen配置 都可 有一个自己独享的默认回落

	if defaultFallbackAddr := iics.inServer.GetFallback(); defaultFallbackAddr != nil {

		targetAddr = *defaultFallbackAddr
		wlc = iics.wrappedConn

	}
	return
}

//被 handshakeInserver_and_passToOutClient 和 handshakeInserver 的innerMux部分 调用，会调用 dialClient_andRelay
func passToOutClient(iics incomingInserverConnState, isfallback bool, wlc net.Conn, udp_wlc netLayer.MsgConn, targetAddr netLayer.Addr) {

	////////////////////////////// 回落阶段 /////////////////////////////////////

	if isfallback {

		fallbackTargetAddr, fallbackWlc := checkfallback(iics)
		if fallbackWlc != nil {
			targetAddr = fallbackTargetAddr
			wlc = fallbackWlc
		}
	}

	if wlc == nil && udp_wlc == nil {
		//无wlc证明 inServer 握手失败，且 没有任何回落可用, 直接退出。
		utils.Debug("invalid request and no matched fallback, hung up")
		if iics.wrappedConn != nil {
			iics.wrappedConn.Close()

		}
		return
	}

	//此时 targetAddr已经完全确定

	////////////////////////////// DNS解析阶段 /////////////////////////////////////

	//dns解析会试图解析域名并将ip放入 targetAddr中
	// 因为在direct时，netLayer.Addr 拨号时，会优先选用ip拨号，而且我们下面的分流阶段 如果使用ip的话，
	// 可以利用geoip文件,  可以做到国别分流.

	if iics.RoutingEnv != nil && iics.RoutingEnv.DnsMachine != nil && (targetAddr.Name != "" && len(targetAddr.IP) == 0) && targetAddr.Network != "unix" {

		if ce := utils.CanLogDebug("Dns querying"); ce != nil {
			ce.Write(zap.String("domain", targetAddr.Name))
		}

		ip := iics.RoutingEnv.DnsMachine.Query(targetAddr.Name)

		if ip != nil {
			targetAddr.IP = ip

			if ce2 := utils.CanLogDebug("Dns result"); ce2 != nil {
				ce2.Write(zap.String("domain", targetAddr.Name), zap.String("ip", ip.String()))
			}
		}
	}

	////////////////////////////// 分流阶段 /////////////////////////////////////

	var client proxy.Client = iics.defaultClient
	routed := false

	inServer := iics.inServer

	//尝试分流, 获取到真正要发向 的 outClient
	if iics.RoutingEnv != nil && iics.RoutingEnv.RoutePolicy != nil && !(inServer != nil && inServer.CantRoute()) {

		desc := &netLayer.TargetDescription{
			Addr: targetAddr,
		}
		if inServer != nil {
			desc.Tag = inServer.GetTag()
		}

		if ce := utils.CanLogDebug("try routing"); ce != nil {
			ce.Write(zap.Any("source", desc))
		}

		outtag := iics.RoutingEnv.RoutePolicy.GetOutTag(desc)

		if outtag == "direct" {
			client = DirectClient
			iics.routedToDirect = true
			routed = true

			if ce := utils.CanLogInfo("Route to direct"); ce != nil {
				ce.Write(
					zap.String("target", targetAddr.UrlString()),
				)
			}
		} else {

			if tagC, ok := ClientsTagMap[outtag]; ok {
				client = tagC
				routed = true
				if ce := utils.CanLogInfo("Route"); ce != nil {
					ce.Write(
						zap.String("to outtag", outtag),
						zap.String("with addr", client.AddrStr()),
						zap.String("and protocol", proxy.GetFullName(client)),
						zap.Any("for source", desc),
					)
				}
			}
		}
	}

	if !routed {
		if ce := utils.CanLogDebug("Default Route"); ce != nil {
			ce.Write(
				zap.Any("source", targetAddr.String()),
				zap.String("client", proxy.GetFullName(client)),
				zap.String("addr", client.AddrStr()),
			)
		}
	}

	////////////////////////////// 特殊处理阶段 /////////////////////////////////////

	// 下面几段用于处理 tls lazy

	var isTlsLazy_clientEnd bool

	if targetAddr.IsUDP() {
		//udp数据是无法splice的，因为不是入口处是真udp就是出口处是真udp; 同样暂不考虑级连情况.
		if iics.isTlsLazyServerEnd {
			iics.isTlsLazyServerEnd = false
			//此时 inServer的tls还被包了一个Recorder，所以我们赶紧关闭记录省着产生额外开销

			iics.inServerTlsRawReadRecorder.StopRecord()
		}
	} else {
		isTlsLazy_clientEnd = Tls_lazy_encrypt && canLazyEncryptClient(client)

	}

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个 就是要发送到 outClient 的信息，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5 对 tls 进行判断
	//
	//  而在服务端探测时，因为 客户端传来的连接 包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 inServer 判断 wlc; 总之，含义就是，去检索“用户承载数据”的来源

	if isTlsLazy_clientEnd || iics.isTlsLazyServerEnd {

		if tlsLayer.PDD {
			log.Printf("loading TLS SniffConn %t %t\n", isTlsLazy_clientEnd, iics.isTlsLazyServerEnd)
		}

		wlc = tlsLayer.NewSniffConn(iics.baseLocalConn, wlc, isTlsLazy_clientEnd, Tls_lazy_secure)

	}

	//这一段代码是去判断是否要在转发结束后自动关闭连接, 主要是socks5 和lazy 的特殊情况

	if targetAddr.IsUDP() {

		if inServer != nil {
			switch inServer.Name() {
			case "socks5":
				// UDP Associate：
				// 因为socks5的 UDP Associate 办法是较为特殊的，不使用现有tcp而是新建立udp，所以此时该tcp连接已经没用了
				// 但是根据socks5标准，这个tcp链接同样是 keep alive的，否则客户端就会认为服务端挂掉了.
				// 另外，此时 targetAddr.IsUDP 只是用于告知此链接是udp Associate，并不包含实际地址信息
			default:
				iics.shouldCloseInSerBaseConnWhenFinish = true

			}
		}

	} else {

		//lazy_encrypt情况比较特殊，基础连接何时被关闭会在tlslazy相关代码中处理。
		// 如果不是lazy的情况的话，转发结束后，要自动关闭
		if !iics.isTlsLazyServerEnd {

			//实测 grpc.Conn 被调用了Close 也不会实际关闭连接，而是会卡住，阻塞，直到底层tcp连接被关闭后才会返回
			// 但是我们还是 直接避免这种情况
			if inServer != nil && !inServer.IsMux() {
				iics.shouldCloseInSerBaseConnWhenFinish = true

			}

		}

	}

	////////////////////////////// 拨号阶段 /////////////////////////////////////

	dialClient_andRelay(iics, targetAddr, client, isTlsLazy_clientEnd, wlc, udp_wlc)
}

//dialClient 对实际client进行拨号，处理传输层, tls层, 高级层等所有层级后，进行代理层握手。
// result = 0 表示拨号成功, result = -1 表示 拨号失败, result = 1 表示 拨号成功 并 已经自行处理了转发阶段(用于lazy和 innerMux )。
// 在 dialClient_andRelay 中被调用。在udp为multi channel时也有用到
func dialClient(targetAddr netLayer.Addr,
	client proxy.Client,
	baseLocalConn,
	wlc net.Conn,
	cachedRemoteAddr string,
	isTlsLazy_clientEnd bool) (

	//return values:
	wrc io.ReadWriteCloser,
	udp_wrc netLayer.MsgConn,
	realTargetAddr netLayer.Addr,
	clientEndRemoteClientTlsRawReadRecorder *tlsLayer.Recorder,
	result int) {

	isudp := targetAddr.IsUDP()

	hasInnerMux := false
	var innerProxyName string

	{
		var muxInt int

		if muxInt, innerProxyName = client.HasInnerMux(); muxInt == 2 {
			hasInnerMux = true

			//先过滤掉 innermux 通道已经建立的情况, 此时我们不必再次外部拨号，而是直接进行内层拨号.
			if client.InnerMuxEstablished() {
				wrc1, realudp_wrc, result1 := dialInnerProxy(client, wlc, nil, innerProxyName, targetAddr, isudp)

				if result1 == 0 {
					if wrc1 != nil {
						wrc = wrc1
					}
					if realudp_wrc != nil {
						udp_wrc = realudp_wrc
					}
					result = result1
					return
				} else {
					utils.Debug("mux failed, will redial")
				}

			}
		}
	}

	var err error
	//先确认拨号地址

	//direct的话自己是没有目的地址的，直接使用 请求的地址
	// 而其它代理的话, realTargetAddr会被设成实际配置的代理的地址
	realTargetAddr = targetAddr

	/*
		if ce := utils.CanLogDebug("request isn't the appointed domain"); ce != nil {


			if uniqueTestDomain != "" && uniqueTestDomain != targetAddr.Name {

				ce.Write(
					zap.String("request", targetAddr.String()),
					zap.String("uniqueTestDomain", uniqueTestDomain),
				)
				result = -1
				return

			}
		}
	*/

	if ce := utils.CanLogInfo("Request"); ce != nil {

		ce.Write(
			zap.String("from", cachedRemoteAddr),
			zap.String("target", targetAddr.UrlString()),
			zap.String("through", proxy.GetVSI_url(client)),
		)
	}

	if client.AddrStr() != "" {

		realTargetAddr, err = netLayer.NewAddr(client.AddrStr())
		if err != nil {

			if ce := utils.CanLogErr("dial client convert addr err"); ce != nil {
				ce.Write(zap.Error(err))
			}
			result = -1
			return
		}
		realTargetAddr.Network = client.Network()
	}
	var clientConn net.Conn

	var grpcClientConn grpc.ClientConn
	var dialedCommonConn any

	dialHere := !(client.Name() == "direct" && isudp)

	// direct 的udp 是自己拨号的,因为要考虑到fullcone。
	//
	// 不是direct的udp的话，也要分情况:
	//如果是单路的, 则我们在此dial, 如果是多路复用, 则不行, 因为要复用同一个连接
	// Instead, 我们要试图从grpc中取出已经拨号好了的 grpc链接
	adv := client.AdvancedLayer()

	if dialHere {

		switch adv {
		case "grpc":

			grpcClientConn = grpc.GetEstablishedConnFor(&realTargetAddr)

			if grpcClientConn != nil {
				//如果有已经建立好的连接，则跳过传输层拨号和tls阶段
				goto advLayerHandshakeStep
			}
		case "quic":
			dialedCommonConn = client.GetQuic_Client().DialCommonConn(false, nil)
			if dialedCommonConn != nil {
				goto advLayerHandshakeStep
			} else {

				result = -1
				return
			}
		}

		clientConn, err = realTargetAddr.Dial()

		if err != nil {
			if err == netLayer.ErrMachineCantConnectToIpv6 {
				//如果一开始就知道机器没有ipv6地址，那么该错误就不是error等级，而是warning等级

				if ce := utils.CanLogWarn("Machine HasNo ipv6 but got ipv6 request"); ce != nil {
					ce.Write(
						zap.String("target", realTargetAddr.String()),
					)
				}

			} else {
				//虽然拨号失败,但是不能认为我们一定有错误, 因为很可能申请的ip本身就是不可达的, 所以不是error等级而是warn等级
				if ce := utils.CanLogWarn("failed dialing"); ce != nil {
					ce.Write(
						zap.String("target", realTargetAddr.String()),
						zap.Error(err),
					)
				}
			}
			result = -1
			return
		}

	}

	////////////////////////////// tls握手阶段 /////////////////////////////////////

	if client.IsUseTLS() {

		if isTlsLazy_clientEnd {

			if Tls_lazy_secure && wlc != nil {
				// 如果使用secure办法，则我们每次不能先拨号，而是要detect用户的首包后再拨号
				// 这种情况只需要客户端操作, 此时我们wrc直接传入原始的 刚拨号好的 tcp连接，即 clientConn

				// 而且为了避免黑客攻击或探测，我们要使用uuid作为特殊指令，此时需要 UserServer和 UserClient

				if uc := client.(proxy.UserClient); uc != nil {
					tryTlsLazyRawCopy(true, uc, nil, targetAddr, clientConn, wlc, nil, true, nil)

				}

				result = 1
				return

			} else {
				clientEndRemoteClientTlsRawReadRecorder = tlsLayer.NewRecorder()
				teeConn := tlsLayer.NewTeeConn(clientConn, clientEndRemoteClientTlsRawReadRecorder)

				clientConn = teeConn
			}
		}

		tlsConn, err2 := client.GetTLS_Client().Handshake(clientConn)
		if err2 != nil {
			if ce := utils.CanLogErr("failed in handshake outClient tls"); ce != nil {
				ce.Write(zap.String("target", targetAddr.String()), zap.Error(err))
			}

			result = -1
			return
		}

		clientConn = tlsConn

	}

	////////////////////////////// header 层 /////////////////////////////////////

	if header := client.HasHeader(); header != nil && adv != "ws" {
		clientConn = &httpLayer.HeaderConn{
			Conn: clientConn,
			H:    header,
		}

	}

	////////////////////////////// 高级层握手阶段 /////////////////////////////////////

advLayerHandshakeStep:

	if adv != "" {
		switch adv {
		case "quic":
			qclient := client.GetQuic_Client()
			clientConn, err = qclient.DialSubConn(dialedCommonConn)
			if err != nil {
				eStr := err.Error()
				if strings.Contains(eStr, "too many") {

					if ce := utils.CanLogDebug("DialSubConn got full session, open another one"); ce != nil {
						ce.Write(
							zap.String("full reason", eStr),
						)
					}

					//第一条连接已满，再开一条session
					dialedCommonConn = qclient.DialCommonConn(true, dialedCommonConn)
					if dialedCommonConn == nil {
						//再dial还是nil，也许是暂时性的网络错误, 先退出
						result = -1
						return
					}

					clientConn, err = qclient.DialSubConn(dialedCommonConn)
					if err != nil {
						if ce := utils.CanLogErr("DialSubConn failed after redialed new session"); ce != nil {
							ce.Write(
								zap.Error(err),
							)
						}
						result = -1
						return
					}
				} else {
					if ce := utils.CanLogErr("DialSubConnFunc failed"); ce != nil {
						ce.Write(
							zap.Error(err),
						)
					}
					result = -1
					return
				}

			}
		case "grpc":
			if grpcClientConn == nil {
				grpcClientConn, err = grpc.ClientHandshake(clientConn, &realTargetAddr)
				if err != nil {
					if ce := utils.CanLogErr("grpc.ClientHandshake failed"); ce != nil {
						ce.Write(zap.Error(err))

					}
					if baseLocalConn != nil {
						baseLocalConn.Close()

					}
					result = -1
					return
				}

			}

			clientConn, err = grpc.DialNewSubConn(client.Path(), grpcClientConn, &realTargetAddr, client.IsGrpcClientMultiMode())
			if err != nil {
				if ce := utils.CanLogErr("grpc.DialNewSubConn failed"); ce != nil {

					ce.Write(zap.Error(err))

					//如果底层tcp连接被关闭了的话，错误会是：
					// rpc error: code = Unavailable desc = connection error: desc = "transport: failed to write client preface: tls: use of closed connection"

				}
				if baseLocalConn != nil {
					baseLocalConn.Close()
				}
				result = -1
				return
			}

		case "ws":
			wsClient := client.GetWS_Client()

			var ed []byte

			if wsClient.UseEarlyData && wlc != nil {
				//若配置了 MaxEarlyDataLen，则我们先读一段;
				edBuf := utils.GetPacket()
				edBuf = edBuf[:ws.MaxEarlyDataLen]
				n, e := wlc.Read(edBuf)
				if e != nil {
					if ce := utils.CanLogErr("failed to read ws early data"); ce != nil {
						ce.Write(zap.Error(e))
					}
					result = -1
					return
				}
				ed = edBuf[:n]

				if ce := utils.CanLogDebug("will send early data"); ce != nil {
					ce.Write(
						zap.Int("len", n),
					)
				}

			}

			// 我们verysimple的架构是 ws握手之后，再进行vless握手
			// 但是如果要传输earlydata的话，则必须要在握手阶段就 预知 vless 的所有数据才行
			// 所以我们需要一种特殊方法

			var wc net.Conn

			if len(ed) > 0 {
				wc, err = wsClient.HandshakeWithEarlyData(clientConn, ed)

			} else {
				wc, err = wsClient.Handshake(clientConn)

			}

			if err != nil {
				if ce := utils.CanLogErr("failed in handshake ws"); ce != nil {
					ce.Write(
						zap.String("target", targetAddr.String()),
						zap.Error(err),
					)
				}
				result = -1
				return
			}

			clientConn = wc
		}
	}

	////////////////////////////// 代理层 握手阶段 /////////////////////////////////////

	if !isudp || hasInnerMux {
		//udp但是有innermux时 依然用handshake, 而不是 EstablishUDPChannel
		var firstPayload []byte

		if !hasInnerMux { //如果有内层mux，要在dialInnerProxy函数里再读
			firstPayload = utils.GetMTU()

			wlc.SetReadDeadline(time.Now().Add(proxy.FirstPayloadTimeout))
			n, err := wlc.Read(firstPayload)
			if err != nil {

				if !errors.Is(err, os.ErrDeadlineExceeded) {
					if ce := utils.CanLogErr("Read first payload failed not because of timeout, will hung up"); ce != nil {
						ce.Write(
							zap.String("target", targetAddr.String()),
							zap.Error(err),
						)
					}

					clientConn.Close()
					wlc.Close()
					result = -1
					return
				} else {
					if ce := utils.CanLogWarn("Read first payload but timeout, will relay without first payload."); ce != nil {
						ce.Write(
							zap.String("target", targetAddr.String()),
							zap.Error(err),
						)
					}
				}

			}
			wlc.SetReadDeadline(time.Time{})
			firstPayload = firstPayload[:n]
		}

		if len(firstPayload) > 0 {
			if ce := utils.CanLogDebug("handshake client with first payload"); ce != nil {
				ce.Write(
					zap.Int("len", len(firstPayload)),
				)
			}
		}

		wrc, err = client.Handshake(clientConn, firstPayload, targetAddr)
		if err != nil {
			if ce := utils.CanLogErr("Handshake client failed"); ce != nil {
				ce.Write(
					zap.String("target", targetAddr.String()),
					zap.Error(err),
				)
			}
			result = -1
			return
		}

	} else {

		udp_wrc, err = client.EstablishUDPChannel(clientConn, targetAddr)
		if err != nil {
			if ce := utils.CanLogErr("EstablishUDPChannel failed"); ce != nil {
				ce.Write(
					zap.String("target", targetAddr.String()),
					zap.Error(err),
				)
			}
			result = -1
			return
		}
	}

	////////////////////////////// 建立内层 mux 阶段 /////////////////////////////////////
	if hasInnerMux {
		//我们目前的实现中，mux统一使用smux v1, 即 smux.DefaultConfig返回的值。这可以兼容trojan的实现。

		wrc, udp_wrc, result = dialInnerProxy(client, wlc, wrc, innerProxyName, targetAddr, isudp)
	}

	return
} //dialClient

//在 dialClient 中调用。 如果调用不成功，则result == -1
func dialInnerProxy(client proxy.Client, wlc net.Conn, wrc io.ReadWriteCloser, innerProxyName string, targetAddr netLayer.Addr, isudp bool) (realwrc io.ReadWriteCloser, realudp_wrc netLayer.MsgConn, result int) {

	smuxSession := client.GetClientInnerMuxSession(wrc)
	if smuxSession == nil {
		result = -1
		utils.Debug("dialInnerProxy return fail 1")
		return
	}

	stream, err := smuxSession.OpenStream()
	if err != nil {
		client.CloseInnerMuxSession() //发现就算 OpenStream 失败, session也不会自动被关闭, 需要我们手动关一下。

		if ce := utils.CanLogDebug("dialInnerProxy return fail 2"); ce != nil {
			ce.Write(zap.Error(err))
		}
		result = -1
		return
	}

	muxDialConf := proxy.DialConf{
		CommonConf: proxy.CommonConf{
			Protocol: innerProxyName,
		},
	}

	muxClient, err := proxy.NewClient(&muxDialConf)
	if err != nil {
		if ce := utils.CanLogDebug("mux inner proxy client creation failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		result = -1
		return
	}
	if isudp {
		realudp_wrc, err = muxClient.EstablishUDPChannel(stream, targetAddr)
		if err != nil {
			if ce := utils.CanLogDebug("mux inner proxy client handshake failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			result = -1
			return
		}
	} else {

		firstPayload := utils.GetMTU()

		wlc.SetReadDeadline(time.Now().Add(proxy.FirstPayloadTimeout))
		n, err := wlc.Read(firstPayload)

		if err != nil {
			if ce := utils.CanLogErr("Read innermux first payload failed"); ce != nil {
				ce.Write(
					zap.String("target", targetAddr.String()),
					zap.Error(err),
				)
			}

			if !errors.Is(err, os.ErrDeadlineExceeded) {
				if ce := utils.CanLogErr("Read innermux first payload failed not because of timeout, will hung up"); ce != nil {
					ce.Write(
						zap.String("target", targetAddr.String()),
						zap.Error(err),
					)
				}

				stream.Close()
				wlc.Close()
				result = -1
				return
			}

		} else {
			if ce := utils.CanLogDebug("innerMux got first payload"); ce != nil {
				ce.Write(
					zap.String("target", targetAddr.String()),
					zap.Int("payloadLen", n),
				)
			}
		}
		wlc.SetReadDeadline(time.Time{})

		realwrc, err = muxClient.Handshake(stream, firstPayload[:n], targetAddr)
		if err != nil {
			if ce := utils.CanLogDebug("mux inner proxy client handshake failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			result = -1
			return
		}
	}

	return
} //dialInnerProxy

// dialClient_andRelay 进行实际转发(Copy)。被 passToOutClient 调用.
// targetAddr为用户所请求的地址。
// client为真实要拨号的client，可能会与iics里的defaultClient不同。以client为准。
// wlc为调用者所提供的 此请求的 来源 链接
func dialClient_andRelay(iics incomingInserverConnState, targetAddr netLayer.Addr, client proxy.Client, isTlsLazy_clientEnd bool, wlc net.Conn, udp_wlc netLayer.MsgConn) {

	//在内层mux时, 不能因为单个传输完毕就关闭整个连接
	if innerMuxResult, _ := client.HasInnerMux(); innerMuxResult == 0 {
		if iics.shouldCloseInSerBaseConnWhenFinish {
			if iics.baseLocalConn != nil {
				defer iics.baseLocalConn.Close()
			}
		}

		if wlc != nil {
			defer wlc.Close()

		}
	}

	wrc, udp_wrc, realTargetAddr, clientEndRemoteClientTlsRawReadRecorder, result := dialClient(targetAddr, client, iics.baseLocalConn, wlc, iics.cachedRemoteAddr, isTlsLazy_clientEnd)
	if result != 0 {
		return
	}

	////////////////////////////// 实际转发阶段 /////////////////////////////////////

	if !targetAddr.IsUDP() {

		if Tls_lazy_encrypt && !iics.routedToDirect {

			// 我们加了回落之后，就无法确定 “未使用tls的outClient 一定是在服务端” 了
			if isTlsLazy_clientEnd {

				if client.IsUseTLS() {
					//必须是 UserClient
					if userClient := client.(proxy.UserClient); userClient != nil {
						tryTlsLazyRawCopy(false, userClient, nil, netLayer.Addr{}, wrc, wlc, iics.baseLocalConn, true, clientEndRemoteClientTlsRawReadRecorder)
						return
					}
				}

			} else if iics.isTlsLazyServerEnd {

				// 最新代码已经确认，使用uuid 作为 “特殊指令”，所以要求Server必须是一个 proxy.UserServer
				// 否则将无法开启splice功能。这是为了防止0-rtt 探测;

				if userServer, ok := iics.inServer.(proxy.UserServer); ok {
					tryTlsLazyRawCopy(false, nil, userServer, netLayer.Addr{}, wrc, wlc, iics.baseLocalConn, false, iics.inServerTlsRawReadRecorder)
					return
				}

			}

		}

		if iics.theFallbackFirstBuffer != nil {
			//这里注意，因为是把 tls解密了之后的数据发送到目标地址，所以这种方式只支持转发到本机纯http服务器
			wrc.Write(iics.theFallbackFirstBuffer.Bytes())
			utils.PutBytes(iics.theFallbackFirstBuffer.Bytes()) //这个Buf不是从utils.GetBuf创建的，而是从一个 GetBytes的[]byte 包装 的，所以我们要PutBytes，而不是PutBuf
		}

		atomic.AddInt32(&ActiveConnectionCount, 1)

		netLayer.Relay(&realTargetAddr, wrc, wlc, &AllDownloadBytesSinceStart, &AllUploadBytesSinceStart)

		atomic.AddInt32(&ActiveConnectionCount, -1)

		return

	} else {

		if iics.theFallbackFirstBuffer != nil {

			udp_wrc.WriteMsgTo(iics.theFallbackFirstBuffer.Bytes(), targetAddr)
			utils.PutBytes(iics.theFallbackFirstBuffer.Bytes())

		}

		atomic.AddInt32(&ActiveConnectionCount, 1)

		if client.IsUDP_MultiChannel() {
			utils.Debug("Relaying UDP with MultiChannel")

			netLayer.RelayUDP_separate(udp_wrc, udp_wlc, &targetAddr, &AllDownloadBytesSinceStart, &AllUploadBytesSinceStart, func(raddr netLayer.Addr) netLayer.MsgConn {
				utils.Debug("Relaying UDP with MultiChannel,dialfunc called")

				_, udp_wrc, _, _, result := dialClient(raddr, client, iics.baseLocalConn, nil, "", false)

				if ce := utils.CanLogDebug("Relaying UDP with MultiChannel, dialfunc call returned"); ce != nil {
					ce.Write(zap.Int("result", result))
				}

				if result == 0 {
					return udp_wrc

				}
				return nil
			})

		} else {
			netLayer.RelayUDP(udp_wrc, udp_wlc, &AllDownloadBytesSinceStart, &AllUploadBytesSinceStart)

		}

		atomic.AddInt32(&ActiveConnectionCount, -1)

		return
	}

}
