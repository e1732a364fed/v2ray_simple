package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy/direct"
	"github.com/hahahrfool/v2ray_simple/proxy/socks5"
	"github.com/hahahrfool/v2ray_simple/proxy/vless"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

var (
	desc = "v2ray_simple, a very simple implementation of V2Ray, 并且在某些地方试图走在v2ray前面"

	configFileName string

	uniqueTestDomain string //有时需要测试到单一网站的流量，此时为了避免其它干扰，需要在这里声明 一下 该域名，然后程序里会进行过滤

	//另外，本作暂时不考虑引入外界log包。依赖越少越好。

	conf         *SimpleConfig
	directClient proxy.Client

	tls_lazy_encrypt bool
	tls_lazy_secure  bool

	routePolicy *netLayer.RoutePolicy

	listenURL string
	dialURL   string

	isServerEnd bool
)

func init() {
	directClient, _ = proxy.ClientFromURL("direct://")

	flag.BoolVar(&tls_lazy_encrypt, "lazy", false, "tls lazy encrypt (splice)")
	flag.BoolVar(&tls_lazy_secure, "ls", false, "tls lazy secure, use special techs to ensure the tls lazy encrypt data can't be detected. Only valid at client end.")

	flag.StringVar(&configFileName, "c", "client.json", "config file name")

	flag.StringVar(&listenURL, "L", "", "listen URL (i.e. the local part in config file), only enbled when config file is not provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL (i.e. the remote part in config file), only enbled when config file is not provided.")

	flag.StringVar(&uniqueTestDomain, "td", "", "test a single domain, like www.domain.com")
}

func printDesc() {
	printVersion()
	proxy.PrintAllServerNames()

	proxy.PrintAllClientNames()

	fmt.Printf("=============== 所有协议均可套tls ================\n")
}

func main() {

	printDesc()

	flag.Parse()

	var err error

	//in config.go
	loadConfig()

	inServer, err := proxy.ServerFromURL(conf.Server_ThatListenPort_Url)
	if err != nil {

		log.Println("can not create local server: ", err)

		os.Exit(-1)
	}
	defer inServer.Stop()

	if !inServer.CantRoute() && conf.Route != nil {

		netLayer.LoadMaxmindGeoipFile("")

		//目前只支持通过 mycountry进行 geoip分流 这一种情况
		routePolicy = netLayer.NewRoutePolicy()
		if conf.Route.MyCountryISO_3166 != "" {
			routePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(conf.Route.MyCountryISO_3166))

		}
	}

	outClient, err := proxy.ClientFromURL(conf.Client_ThatDialRemote_Url)
	if err != nil {
		log.Println("can not create remote client: ", err)
		os.Exit(-1)
	}
	isServerEnd = outClient.Name() == "direct"

	listener, err := net.Listen("tcp", inServer.AddrStr())
	if err != nil {
		log.Println("can not listen on", inServer.AddrStr(), err)
		os.Exit(-1)
	}

	if utils.CanLogInfo() {
		log.Println(inServer.Name(), "is listening TCP on ", inServer.AddrStr())

	}

	// 后台运行主代码，而main函数只监听中断信号
	// TODO: 未来main函数可以推出 交互模式，等未来推出动态增删用户、查询流量等功能时就有用
	go func() {
		for {
			lc, err := listener.Accept()
			if err != nil {
				errStr := err.Error()
				if strings.Contains(errStr, "closed") {
					log.Println("local connection closed", err)
					break
				}
				log.Println("failed to accepted connection: ", err)
				if strings.Contains(errStr, "too many") {
					log.Println("To many incoming conn! Sleep ", errStr)
					time.Sleep(time.Millisecond * 500)
				}
				continue
			}

			go handleNewIncomeConnection(inServer, outClient, lc)

		}
	}()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}

func handleNewIncomeConnection(inServer proxy.Server, outClient proxy.Client, thisLocalConnectionInstance net.Conn) {

	baseLocalConn := thisLocalConnectionInstance

	if utils.CanLogInfo() {
		log.Println("Got new", thisLocalConnectionInstance.RemoteAddr().String())

	}

	var err error

	//此时，baseLocalConn里面 正常情况下, 服务端看到的是 客户端的golang的tls 拨号发出的 tls数据
	// 客户端看到的是 socks5的数据， 我们首先就是要看看socks5里的数据是不是tls，而socks5自然 IsUseTLS 是false

	// 如果是服务端的话，那就是 inServer.IsUseTLS == true, 此时，我们正常握手，然后我们需要判断的是它承载的数据

	// 每次tls试图从 原始连接 读取内容时，都会附带把原始数据写入到 这个 Recorder中
	var serverEndLocalServerTlsRawReadRecorder *tlsLayer.Recorder

	if inServer.IsUseTLS() {

		if tls_lazy_encrypt {
			serverEndLocalServerTlsRawReadRecorder = tlsLayer.NewRecorder()

			serverEndLocalServerTlsRawReadRecorder.StopRecord() //先不记录，因为一开始是我们自己的tls握手包，没有意义
			teeConn := tlsLayer.NewTeeConn(baseLocalConn, serverEndLocalServerTlsRawReadRecorder)

			thisLocalConnectionInstance = teeConn
		}

		tlsConn, err := inServer.GetTLS_Server().Handshake(thisLocalConnectionInstance)
		if err != nil {

			if utils.CanLogErr() {
				log.Println("failed in handshake inServer tls", inServer.AddrStr(), err)

			}
			thisLocalConnectionInstance.Close()
			return
		}

		if tls_lazy_encrypt {
			//此时已经握手完毕，可以记录了
			serverEndLocalServerTlsRawReadRecorder.StartRecord()
		}

		thisLocalConnectionInstance = tlsConn

	}
	//isfallback := false
	var theFallback httpLayer.Fallback

	wlc, targetAddr, err := inServer.Handshake(thisLocalConnectionInstance)
	if err != nil {

		if utils.CanLogWarn() {
			log.Println("failed in handshake from", inServer.AddrStr(), err)
		}

		if inServer.CanFallback() {
			fe, ok := err.(httpLayer.FallbackErr)
			if ok {
				f := fe.Fallback()
				theFallback = f
				if httpLayer.HasFallbackType(f.SupportType(), httpLayer.FallBack_default) {
					targetAddr = f.GetFallback(httpLayer.FallBack_default, "")

					wlc = thisLocalConnectionInstance
					//isfallback = true
					goto afterLocalServerHandshake
				}

			}
		}

		thisLocalConnectionInstance.Close()
		return
	}

afterLocalServerHandshake:

	var client proxy.Client = outClient

	var routedToDirect bool

	//如果可以route
	if !inServer.CantRoute() && routePolicy != nil {

		if utils.CanLogInfo() {
			log.Println("trying routing feature")
		}

		//目前只支持一个 inServer/outClient, 所以目前根据tag分流是没有意义的，以后再说
		// 现在就用addr分流就行
		outtag := routePolicy.GetOutTag(&netLayer.TargetDescription{
			Addr: targetAddr,
		})
		if outtag == "direct" {
			client = directClient
			routedToDirect = true

			if utils.CanLogInfo() {
				log.Println("routed to direct", targetAddr.UrlString())
			}
		}
	}

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个 就是要发送到 outClient 的信息，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5 对 tls 进行判断
	//
	//  而在服务端探测时，因为 客户端传来的连接 包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 inServer 判断 wlc; 总之，含义就是，去检索“用户承载数据”的来源

	if tls_lazy_encrypt && !(!isServerEnd && routedToDirect) {

		if tlsLayer.PDD {
			log.Println("loading TLS SniffConn", !isServerEnd)
		}

		wlc = tlsLayer.NewSniffConn(baseLocalConn, wlc, !isServerEnd, tls_lazy_secure)

		//clientConn = cc
	}

	//如果目标是udp则要分情况讨论
	if targetAddr.IsUDP {

		switch inServer.Name() {
		case "vlesss":
			fallthrough
		case "vless":

			if targetAddr.Name == vless.CRUMFURS_Established_Str {
				// 预留了 CRUMFURS 信道的话，就不要关闭 thisLocalConnectionInstance
				// 	而且也不在这里处理监听事件，client自己会在额外的 goroutine里处理
				//	server也一样，会在特定的场合给 CRUMFURS 传值，这个机制是与main函数无关的

				// 而且 thisLocalConnectionInstance 会被 inServer 保存起来，用于后面的 unknownRemoteAddrMsgWriter

				return

			} else {
				//如果不是CRUMFURS命令，那就是普通的针对某udp地址的连接，见下文 uniExtractor 的使用
				defer thisLocalConnectionInstance.Close()
			}

		case "socks5":

			// UDP Associate：
			//
			// 因为socks5的 UDP Associate 办法是较为特殊的（不使用现有tcp而是新建立udp），所以要单独拿出来处理。
			// 此时 targetAddr.IsUDP 只是用于告知此链接是udp Associate，并不包含实际地址信息

			defer thisLocalConnectionInstance.Close()

			// 此时socks5包已经帮我们dial好了一个udp连接，即wlc，但是还未读取到客户端想要访问的东西
			udpConn := wlc.(*socks5.UDPConn)

			// 将 outClient 视为 UDP_Putter ，就可以转发udp信息了
			// direct 也实现了 UDP_Putter (通过 UDP_Pipe和 RelayUDP_to_Direct函数), 所以目前 socks5直接转发udp到direct 的功能 已经实现。

			// 我在 vless v1 的client 的 UserConn 中实现了 UDP_Putter, vless 的 client的 新连接的Handshake过程会在 UDP_Putter.WriteUDPRequest 被调用 时发生

			if putter := outClient.(proxy.UDP_Putter); putter != nil {

				//UDP_Putter 不使用传统的Handshake过程，因为Handshake是用于第一次数据，然后后面接着的双向传输都不再需要额外信息；而 UDP_Putter 每一次数据传输都是需要传输 目标地址的，所以每一次都需要一些额外数据，这就是我们 UDP_Putter 接口去解决的事情。

				//因为UDP Associate后，就会保证以后的向 wlc 的 所有请求数据都是udp请求，所以可以在这里直接循环转发了。

				go udpConn.StartPushResponse(putter)
				udpConn.StartReadRequest(putter)

			}
			return

		default:
			defer thisLocalConnectionInstance.Close()

		}
	} else {
		if !tls_lazy_encrypt { //lazy_encrypt情况比较特殊
			defer thisLocalConnectionInstance.Close()

		}

		// 这里 因为 vless v1 的 CRUMFURS 信道 会对 thisLocalConnectionInstance 出现keep alive ，
		// 而且具体的传递信息的部分并不是在main函数中处理，而是自己的go routine，所以不能直接关闭 thisLocalConnectionInstance，
		// 所以要分情况进行 defer thisLocalConnectionInstance.Close()。
	}

	// 如果目标是udp 则我们单独处理。这里为了简便，不考虑多级串联的关系，直接只考虑 直接转发到direct
	// 根据 vless_v1的讨论，vless_v1 的udp转发的 通信方式 也是与tcp连接类似的分离信道方式
	//	上面已经把 CRUMFURS 的情况过滤掉了，所以现在这里就是普通的udp请求
	//
	// 因为direct使用 proxy.RelayUDP_to_Direct 函数 直接实现了fullcone
	// 那么我们只需要传入一个  UDP_Extractor 即可

	if targetAddr.IsUDP {

		var unknownRemoteAddrMsgWriter proxy.UDPResponseWriter

		switch inServer.Name() {
		case "vlesss":
			fallthrough
		case "vless":

			uc := wlc.(*vless.UserConn)

			if uc.GetProtocolVersion() < 1 {
				break
			}

			//unknownRemoteAddrMsgWriter 在 vless v1中的实现就是 theCRUMFURS （vless v0就是mux）

			id := uc.GetIdentityStr()

			vlessServer := inServer.(*vless.Server)

			theCRUMFURS := vlessServer.Get_CRUMFURS(id)

			unknownRemoteAddrMsgWriter = theCRUMFURS
		}

		uniExtractor := proxy.NewUniUDP_Extractor(targetAddr.ToUDPAddr(), wlc, unknownRemoteAddrMsgWriter)

		direct.RelayUDP_to_Direct(uniExtractor) //阻塞

		return
	}

	var realTargetAddr *netLayer.Addr = targetAddr //direct的话自己是没有目的地址的，直接使用 请求的地址

	if uniqueTestDomain != "" && uniqueTestDomain != targetAddr.Name {
		if utils.CanLogDebug() {
			log.Println("request isn't the appointed domain", targetAddr, uniqueTestDomain)

		}
		return
	}

	if utils.CanLogInfo() {
		log.Println(client.Name(), " want to dial ", targetAddr.UrlString())

	}

	if client.AddrStr() != "" {
		//log.Println("will dial", client.AddrStr())

		realTargetAddr, _ = netLayer.NewAddr(client.AddrStr())
	}

	clientConn, err := realTargetAddr.Dial()
	if err != nil {
		if utils.CanLogErr() {
			log.Println("failed in dial", targetAddr.String(), ", Reason: ", err)

		}
		return
	}

	var clientEndRemoteClientTlsRawReadRecorder *tlsLayer.Recorder

	if client.IsUseTLS() { //即客户端

		if tls_lazy_encrypt {

			if tls_lazy_secure {
				// 如果使用secure办法，则我们每次不能先拨号，而是要detect用户的首包后再拨号
				// 这种情况只需要客户端操作, 此时我们wrc直接传入原始的 刚拨号好的 tcp连接，即 clientConn

				// 而且为了避免黑客攻击或探测，我们要使用uuid作为特殊指令，此时需要 UserServer和 UserClient

				if uc := client.(proxy.UserClient); uc != nil {
					tryRawCopy(true, uc, nil, targetAddr, clientConn, wlc, nil, true, nil)

				}

				return

			} else {
				clientEndRemoteClientTlsRawReadRecorder = tlsLayer.NewRecorder()
				teeConn := tlsLayer.NewTeeConn(clientConn, clientEndRemoteClientTlsRawReadRecorder)

				clientConn = teeConn
			}
		}

		tlsConn, err := client.GetTLS_Client().Handshake(clientConn)
		if err != nil {
			log.Println("failed in handshake outClient tls", targetAddr.String(), ", Reason: ", err)
			return
		}

		clientConn = tlsConn

	}

	wrc, err := client.Handshake(clientConn, targetAddr)
	if err != nil {
		if utils.CanLogErr() {
			log.Println("failed in handshake to", targetAddr.String(), ", Reason: ", err)

		}
		return
	}

	if !routedToDirect && tls_lazy_encrypt {

		// 这里的错误是，我们加了回落之后，就无法确定 “未使用tls的outClient 一定是在服务端” 了
		if !isServerEnd {

			if client.IsUseTLS() {
				//必须是 UserClient
				if userClient := client.(proxy.UserClient); userClient != nil {
					tryRawCopy(false, userClient, nil, nil, wrc, wlc, baseLocalConn, true, clientEndRemoteClientTlsRawReadRecorder)
					return
				}
			}

		} else {

			// 最新代码已经确认，使用uuid 作为 “特殊指令”，所以要求Server必须是一个 proxy.UserServer
			// 否则将无法开启splice功能。这是为了防止0-rtt 探测;

			if userServer, ok := inServer.(proxy.UserServer); ok {
				tryRawCopy(false, nil, userServer, nil, wrc, wlc, baseLocalConn, false, serverEndLocalServerTlsRawReadRecorder)
				return
			}

		}

	}

	if theFallback != nil {
		//这里注意，因为是吧tls解密了之后的数据发送到目标地址，所以这种方式只支持转发到本机纯http服务器
		wrc.Write(theFallback.FirstBuffer().Bytes())
		utils.PutBytes(theFallback.FirstBuffer().Bytes()) //这个Buf不是从utils.GetBuf创建的，而是从一个 GetBytes的[]byte 包装 的
	}

	/*
		// debug时可以使用这段代码
		go func() {
			n, e := io.Copy(wrc, wlc)
			log.Println("本地->远程 转发结束", realTargetAddr.String(), n, e)
		}()
		n, e := io.Copy(wlc, wrc)

		log.Println("远程->本地 转发结束", realTargetAddr.String(), n, e)
	*/

	go io.Copy(wrc, wlc)
	io.Copy(wlc, wrc)
}

// tryRawCopy 尝试能否直接对拷，对拷 直接使用 原始 TCPConn，也就是裸奔转发
//  如果在linux上，则和 xtls的splice 含义相同. 在其他系统时，与xtls-direct含义相同。
// 我们内部先 使用 DetectConn进行过滤分析，然后再判断进化为splice 或者退化为普通拷贝
// 第一个参数仅用于 tls_lazy_secure
func tryRawCopy(useSecureMethod bool, proxy_client proxy.UserClient, proxy_server proxy.UserServer, clientAddr *netLayer.Addr, wrc, wlc io.ReadWriter, localConn net.Conn, isclient bool, theRecorder *tlsLayer.Recorder) {

	//如果用了 lazy_encrypt， 则不直接利用Copy，因为有两个阶段：判断阶段和直连阶段
	// 在判断阶段，因为还没确定是否是 tls，所以是要继续用tls加密的，
	// 而直连阶段，只要能让 Copy使用 net.TCPConn的 ReadFrom, 就不用管了， golang最终就会使用splice
	// 之所以可以对拷直连，是因为无论是 socks5 还是vless，只是在最开始的部分 加了目标头，后面的所有tcp连接都是直接传输的数据，就是说，一开始握手什么的是不能直接对拷的，等到后期就可以了
	// 而且之所以能对拷，还有个原因就是，远程服务器 与 客户端 总是源源不断地 为 我们的 原始 TCP 连接 提供数据，我们只是一个中间商而已，左手倒右手

	// 如果开启了  half lazy 开关，则会在 Write的那一端 加强过滤，过滤一些alert(目前还没做)，然后 只在Read端 进行splice
	//
	// 如果是客户端，则 从 wlc 读取，写入 wrc ，这种情况是 Write, 然后对于 DetectConn 来说是 Read，即 从DetectConn读取，然后 写入到远程连接
	// 如果是服务端，则 从 wrc 读取，写入 wlc， 这种情况是 Write
	//
	// 总之判断 Write 的对象，是考虑 客户端和服务端之间的数据传输，不考虑 远程真实服务器

	wlcdc := wlc.(*tlsLayer.SniffConn)
	wlccc_raw := wlcdc.RawConn

	if isclient {
		sc := []byte(proxy_client.GetUser().GetIdentityBytes())
		wlcdc.R.SpecialCommandBytes = sc
		wlcdc.W.SpecialCommandBytes = sc
	} else {
		wlcdc.R.UH = proxy_server
	}

	var rawWRC *net.TCPConn

	if !useSecureMethod {

		//wrc 有两种情况，如果客户端那就是tls，服务端那就是direct。我们不讨论服务端 处于中间层的情况

		if isclient {
			// 不过实际上客户端 wrc 是 vless的 UserConn， 而UserConn的底层连接才是TLS
			// 很明显，目前我们只支持vless所以才可这么操作，以后再说。

			wrcVless := wrc.(*vless.UserConn)
			tlsConn := wrcVless.Conn.(*tlsLayer.Conn)
			rawWRC = tlsConn.GetRaw(tls_lazy_encrypt)

			//不过仔细思考，我们根本不需要这么繁琐地获取啊？！因为我们的 原始连接我们本来就是有的！
			//rawWRC = localConn.(*net.TCPConn) //然而我实测，竟然传输会得到错误的结果，怎么回事

		} else {
			rawWRC = wrc.(*net.TCPConn) //因为是direct
		}

		if rawWRC == nil {
			if tlsLayer.PDD {
				log.Println("splice fail reason 0 ")

			}

			if tls_lazy_encrypt {
				theRecorder.StopRecord()
				theRecorder.ReleaseBuffers()
			}

			//退化回原始状态
			go io.Copy(wrc, wlc)
			io.Copy(wlc, wrc)
			return
		}
	} else {
		rawWRC = wrc.(*net.TCPConn) //useSecureMethod的一定是客户端，此时就是直接给出原始连接
	}

	waitWRC_CreateChan := make(chan int)

	go func(wrcPtr *io.ReadWriter) {
		//从 wlccc 读取，向 wrc 写入
		// 此时如果ReadFrom，那就是 wrc.ReadFrom(wlccc)
		//wrc 要实现 ReaderFrom才行, 或者把最底层TCPConn暴露，然后 wlccc 也要把最底层 TCPConn暴露出来
		// 这里就直接采取底层方式

		p := utils.GetPacket()
		isgood := false
		isbad := false

		checkCount := 0

		for {
			if isgood || isbad {
				break
			}
			n, err := wlcdc.Read(p)
			if err != nil {
				break
			}

			checkCount++

			if useSecureMethod && checkCount == 1 {
				//此时还未dial，需要进行dial; 仅限客户端

				if tlsLayer.PDD {
					log.Println(" 才开始Dial 服务端")

				}

				theRecorder = tlsLayer.NewRecorder()
				teeConn := tlsLayer.NewTeeConn(rawWRC, theRecorder)

				tlsConn, err := proxy_client.GetTLS_Client().Handshake(teeConn)
				if err != nil {
					if utils.CanLogErr() {
						log.Println("failed in handshake outClient tls", ", Reason: ", err)

					}
					return
				}

				wrc, err = proxy_client.Handshake(tlsConn, clientAddr)
				if err != nil {
					if utils.CanLogErr() {
						log.Println("failed in handshake to", clientAddr.String(), ", Reason: ", err)
					}
					return
				}

				*wrcPtr = wrc

				waitWRC_CreateChan <- 1

			} else {
				if tlsLayer.PDD {
					log.Println(" 第", checkCount, "次测试")
				}
			}

			//wrc.Write(p[:n])
			//在判断 “是TLS” 的瞬间，它会舍弃写入数据，而把写入的主动权交回我们，我们发送特殊命令后，通过直连写入数据
			if wlcdc.R.IsTls && wlcdc.RawConn != nil {
				isgood = true

				if isclient {

					// 若是client，因为client是在Read时判断的 IsTLS，所以特殊指令实际上是要在这里发送

					if tlsLayer.PDD {
						log.Println("R 准备发送特殊命令, 以及保存的TLS内容", len(p[:n]))
					}

					wrc.Write(wlcdc.R.SpecialCommandBytes)

					//然后还要发送第一段FreeData

					rawWRC.Write(p[:n])

				} else {

					//如果是 server, 则 此时   已经收到了解密后的 特殊指令
					// 我们要从 theRecorder 中最后一个Buf里找 原始数据

					//theRecorder.DigestAll()

					//这个瞬间，R是存放了 SpecialCommand了（即uuid），然而W还是没有的 ，
					// 所以我们要先给W的SpecialCommand 赋值

					wlcdc.W.SpecialCommandBytes = wlcdc.R.SpecialCommandBytes

					rawBuf := theRecorder.GetLast()
					bs := rawBuf.Bytes()

					/*
						if tlsLayer.PDD {
							_, record_count := tlsLayer.GetLastTlsRecordTailIndex(bs)
							if record_count < 2 { // 应该是0-rtt的情况

								log.Println("检测到0-rtt"")

							}
							log.Println("R Recorder 中记录了", record_count, "条记录")
						}
					*/

					nextI := tlsLayer.GetTlsRecordNextIndex(bs)

					//有可能只存有一个record，然后 supposedLen非常长，此时 nextI是大于整个bs长度的
					//正常来说这是不应该发生的，但是实际测速时发生了！会导致服务端闪退，
					// 就是说在客户端上传大流量时，可能导致服务端出问题
					//
					//仔细思考，如果在客户端发送特殊指令的同时，tls的Conn仍然在继续写入的话，那么就有可能出现这种情况，
					// 也就是说，是多线程问题；但是还是不对，如果tls正在写入，那么我们则还没达到写特殊指令的代码
					//只能说，写入的顺序完全是正确的，但是收到的数据似乎有错误发生
					//
					// 还是说，特殊指令实际上被分成了两个record发送？这么短的指令也能吗？
					//还有一种可能？就是在写入“特殊指令”的瞬间，需要发送一些alert？然后在特殊指令的前后一起发送了？
					//仅在 ubuntu中测试发生过，macos中 测试从未发生过

					//总之，实际测试这个 nextI 似乎特别大，然后bs也很大。bs大倒是正常，因为是测速
					//
					// 一种情况是，特殊指令粘在上一次tls包后面被一起发送。那么此时lastbuffer应该完全是新自由数据
					// 上一次的tls包应该是最后一个握手包。但是问题是，client必须要收到服务端的握手回应才能继续发包
					// 所以还是不应该发生。
					//  除非，使用了某种方式在握手的同时也传递数据，等等，tls1.3的0-rtt就是如此啊！
					//
					// 而且，上传正好属于握手的同时上传数据的情况。
					// 而服务端是无法进行tls1.3的0-rtt的，因为理论上 tls1.3的 0-rtt只能由客户端发起。
					// 所以才会出现下载时毫无问题，上传时出bug的情况
					//
					//如果是0-rtt，我们的Recorder应该根本没有记录到我们的特殊指令包，因为它是从第二个包开始记录的啊！
					// 所以我们从Recorder获取到的包是不含“特殊指令”包的，所以获取到的整个数据全是我们想要的

					if len(bs) < nextI {
						// 应该是 tls1.3 0-rtt的情况

						rawWRC.Write(bs)

					} else {
						nextFreeData := bs[nextI:]

						if tlsLayer.PDD {
							log.Println("R 从Recorder 提取 真实TLS内容", len(nextFreeData))

						}
						rawWRC.Write(nextFreeData)

					}

					theRecorder.StopRecord()
					theRecorder.ReleaseBuffers()

				}
			} else {

				if tlsLayer.PDD {
					log.Println("pass write")

				}
				wrc.Write(p[:n])
				if wlcdc.R.DefinitelyNotTLS {
					isbad = true
				}
			}
		}
		utils.PutPacket(p)

		if isbad {
			//直接退化成普通Copy

			if tlsLayer.PDD {
				log.Println("SpliceRead R方向 退化……", wlcdc.R.GetFailReason())
			}
			io.Copy(wrc, wlc)
			return
		}

		if isgood {
			if runtime.GOOS == "linux" {
				runtime.Gosched() //详情请阅读我的 xray_splice- 文章，了解为什么这个调用是必要的
			}

			if tlsLayer.PDD {
				log.Println("成功SpliceRead R方向")
				num, e1 := rawWRC.ReadFrom(wlccc_raw)
				log.Println("SpliceRead R方向 传完，", e1, ", 长度:", num)
			} else {
				rawWRC.ReadFrom(wlccc_raw)
			}

		}
	}(&wrc)

	isgood2 := false
	isbad2 := false

	p := utils.GetPacket()

	count := 0

	//从 wrc  读取，向 wlccc 写入
	for {
		if isgood2 || isbad2 {
			break
		}

		count++

		if useSecureMethod && count == 1 {
			<-waitWRC_CreateChan
		}
		if tlsLayer.PDD {
			log.Println("准备从wrc读")
		}
		n, err := wrc.Read(p)
		if err != nil {
			break
		}

		if tlsLayer.PDD {
			log.Println("从wrc读到数据，", n, "准备写入wlcdc")
		}

		wn, _ := wlcdc.Write(p[:n])

		if tlsLayer.PDD {
			log.Println("写入wlcdc完成", wn)
		}

		if wlcdc.W.IsTls && wlcdc.RawConn != nil {

			if isclient {
				//读到了服务端 发来的 特殊指令

				rawBuf := theRecorder.GetLast()
				bs := rawBuf.Bytes()

				nextI := tlsLayer.GetTlsRecordNextIndex(bs)
				if nextI > len(bs) { //理论上有可能，但是又不应该，收到的buf不应该那么短，应该至少包含一个有效的整个tls record，因为此时理论上已经收到了服务端的 特殊指令，它是单独包在一个 tls record 里的

					//不像上面类似的一段，这个例外从来没有被触发过，也就是说，下载方向是毫无问题的
					//
					//这是因为, 触发上一段类似代码的原因是tls 0-rtt，而 tls 0-rtt 总是由 客户端发起的
					log.Println("有问题， nextI > len(bs)", nextI, len(bs))
					//os.Exit(-1) //这里就暂时不要退出程序了，毕竟理论上有可能由一些黑客来触发这里。
					localConn.Close()
					rawWRC.Close()
					return
				}

				if nextI < len(bs) {
					//有额外的包
					nextFreeData := bs[nextI:]

					wlccc_raw.Write(nextFreeData)
				}

				theRecorder.StopRecord()
				theRecorder.ReleaseBuffers()

			} else {

				//此时已经写入了 特殊指令，需要再发送 freedata

				wlccc_raw.Write(p[:n])
			}

			isgood2 = true

		} else if wlcdc.W.DefinitelyNotTLS {
			isbad2 = true
		}
	}
	utils.PutPacket(p)

	if isbad2 {

		if tlsLayer.PDD {
			log.Println("SpliceRead W方向 退化……", wlcdc.W.GetFailReason())
		}
		io.Copy(wlc, wrc)
		return
	}

	if isgood2 {
		if tlsLayer.PDD {
			log.Println("成功SpliceRead W方向,准备 直连对拷")
		}

		if runtime.GOOS == "linux" {
			runtime.Gosched() //详情请阅读我的 xray_splice- 文章，了解为什么这个调用是必要的。不过不一定对，因为我很笨，只是看xray的代码有这一行

		}
		if tlsLayer.PDD {

			num, e2 := wlccc_raw.ReadFrom(rawWRC) //看起来是ReadFrom，实际上是向 wlccc_raw进行Write，即箭头向左
			log.Println("SpliceRead W方向 传完，", e2, ", 长度:", num)
		} else {
			wlccc_raw.ReadFrom(rawWRC)
		}

	}

}
