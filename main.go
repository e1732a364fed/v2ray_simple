package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/grpc"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/quic"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/hahahrfool/v2ray_simple/ws"

	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/proxy/socks5"
	"github.com/hahahrfool/v2ray_simple/proxy/vless"

	_ "github.com/hahahrfool/v2ray_simple/proxy/direct"
	_ "github.com/hahahrfool/v2ray_simple/proxy/dokodemo"
	_ "github.com/hahahrfool/v2ray_simple/proxy/http"
)

const (
	simpleMode = iota
	standardMode
	v2rayCompatibleMode
)

var (
	configFileName string

	uniqueTestDomain string //有时需要测试到单一网站的流量，此时为了避免其它干扰，需要在这里声明 一下 该域名，然后程序里会进行过滤

	confMode        int = -1 //0: simple json, 1: standard toml, 2: v2ray compatible json
	simpleConf      *proxy.Simple
	standardConf    *proxy.Standard
	directClient, _ = proxy.ClientFromURL("direct://")
	default_uuid    string

	allServers = make([]proxy.Server, 0, 8)
	allClients = make([]proxy.Client, 0, 8)

	serversTagMap = make(map[string]proxy.Server)
	clientsTagMap = make(map[string]proxy.Client)

	listenURL string //用于命令行模式
	dialURL   string //用于命令行模式

	tls_lazy_encrypt bool
	tls_lazy_secure  bool

	routePolicy  *netLayer.RoutePolicy
	mainFallback *httpLayer.ClassicFallback

	startPProf bool
)

func init() {

	flag.BoolVar(&startPProf, "pp", false, "pprof")

	flag.BoolVar(&tls_lazy_encrypt, "lazy", false, "tls lazy encrypt (splice)")
	flag.BoolVar(&tls_lazy_secure, "ls", false, "tls lazy secure, use special techs to ensure the tls lazy encrypt data can't be detected. Only valid at client end.")

	flag.StringVar(&configFileName, "c", "client.toml", "config file name")

	flag.StringVar(&listenURL, "L", "", "listen URL (i.e. the local part in config file), only enbled when config file is not provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL (i.e. the remote part in config file), only enbled when config file is not provided.")

	flag.StringVar(&uniqueTestDomain, "td", "", "test a single domain, like www.domain.com")

}

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func main() {

	printVersion()

	flag.Parse()

	if startPProf {
		f, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

	}

	utils.AdjustBufSize()

	ll_beforeLoadConfigFile := utils.LogLevel
	usereadv_beforeLoadConfigFile := netLayer.UseReadv

	cmdLL_given := isFlagPassed("ll")
	cmdUseReadv_given := isFlagPassed("readv")

	loadConfig()

	if confMode < 0 {
		log.Fatal("no config exist")
	}

	//有点尴尬, 读取配置文件必须要用命令行参数，而配置文件里的部分配置又会覆盖部分命令行参数

	if cmdLL_given && utils.LogLevel != ll_beforeLoadConfigFile {
		//配置文件配置了日志等级, 但是因为 命令行给出的值优先, 所以要设回

		utils.LogLevel = ll_beforeLoadConfigFile
	}

	if cmdUseReadv_given && netLayer.UseReadv != usereadv_beforeLoadConfigFile {
		//配置文件配置了readv, 但是因为 命令行给出的值优先, 所以要设回

		netLayer.UseReadv = usereadv_beforeLoadConfigFile
	}

	fmt.Println("Log Level:", utils.LogLevel)
	fmt.Println("UseReadv:", netLayer.UseReadv)
	if utils.CanLogDebug() {
		fmt.Println("MaxBufSize", utils.MaxBufLen)
	}

	runPreCommands()

	var err error

	var defaultInServer proxy.Server

	//load server and routePolicy
	switch confMode {
	case simpleMode:
		defaultInServer, err = proxy.ServerFromURL(simpleConf.Server_ThatListenPort_Url)
		if err != nil {
			log.Fatalln("can not create local server: ", err)
		}

		if !defaultInServer.CantRoute() && simpleConf.Route != nil {

			netLayer.LoadMaxmindGeoipFile("")

			//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
			routePolicy = netLayer.NewRoutePolicy()
			if simpleConf.MyCountryISO_3166 != "" {
				routePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(simpleConf.MyCountryISO_3166))

			}
		}
	case standardMode:
		//虽然标准模式支持多个Server，目前先只考虑一个
		//多个Server存在的话，则必须要用 tag指定路由; 然后，我们需在预先阶段就判断好tag指定的路由

		if len(standardConf.Listen) < 1 {
			log.Fatal("no Listen in config settings!")
		}

		for _, serverConf := range standardConf.Listen {
			thisConf := serverConf

			if thisConf.Uuid == "" && default_uuid != "" {
				thisConf.Uuid = default_uuid
			}

			thisServer, err := proxy.NewServer(thisConf)
			if err != nil {
				log.Fatalln("can not create local server: ", err)
			}

			allServers = append(allServers, thisServer)
			if tag := thisServer.GetTag(); tag != "" {
				serversTagMap[tag] = thisServer
			}
		}

		hasMyCountry := (standardConf.App != nil && standardConf.App.MyCountryISO_3166 != "")

		if standardConf.Route != nil || hasMyCountry {

			netLayer.LoadMaxmindGeoipFile("")

			routePolicy = netLayer.NewRoutePolicy()
			if hasMyCountry {
				routePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

			}

			proxy.LoadRulesForRoutePolicy(standardConf.Route, routePolicy)
		}

	}

	var defaultOutClient proxy.Client

	// load client
	switch confMode {
	case simpleMode:
		defaultOutClient, err = proxy.ClientFromURL(simpleConf.Client_ThatDialRemote_Url)
		if err != nil {
			log.Fatalln("can not create remote client: ", err)
		}
	case standardMode:

		if len(standardConf.Dial) < 1 {
			log.Fatal("no dial in config settings!")
		}

		for _, thisConf := range standardConf.Dial {
			if thisConf.Uuid == "" && default_uuid != "" {
				thisConf.Uuid = default_uuid
			}

			thisClient, err := proxy.NewClient(thisConf)
			if err != nil {
				log.Fatalln("can not create remote client: ", err)
			}
			allClients = append(allClients, thisClient)

			if tag := thisClient.GetTag(); tag != "" {
				clientsTagMap[tag] = thisClient
			}
		}

		defaultOutClient = allClients[0]
	}

	// 后台运行主代码，而main函数只监听中断信号
	// TODO: 未来main函数可以推出 交互模式，等未来推出动态增删用户、查询流量等功能时就有用;
	//  或可用于交互生成自己想要的配置
	if confMode == simpleMode {
		listenSer(defaultInServer, defaultOutClient)
	} else {
		for _, inServer := range allServers {
			listenSer(inServer, defaultOutClient)
		}
	}

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}

//非阻塞
func listenSer(inServer proxy.Server, defaultOutClientForThis proxy.Client) {

	//quic
	if inServer.IsHandleInitialLayers() {
		//如果像quic一样自行处理传输层至tls层之间的部分，则我们跳过 handleNewIncomeConnection 函数
		// 拿到连接后直接调用 handshakeInserver_and_passToOutClient

		handleFunc := inServer.HandleInitialLayersFunc()
		if handleFunc == nil {
			log.Fatal("inServer.IsHandleInitialLayers but inServer.HandleInitialLayersFunc() returns nil")
		}

		newConnChan, baseConn := handleFunc()
		if newConnChan == nil {
			//baseConn可以为nil，quic就是如此
			if utils.CanLogErr() {
				log.Println("StarthandleInitialLayers can't extablish baseConn")
			}
			return
		}

		go func() {
			for {
				newConn, ok := <-newConnChan
				if !ok {
					if utils.CanLogErr() {
						log.Println("read from SuperProxy not ok")

					}

					quic.CloseBaseConn(baseConn, "quic")

					return
				}

				iics := incomingInserverConnState{
					wrappedConn: newConn,
					//baseLocalConn: baseConn,	//quic是没有baseLocalConn的，因为基于udp
					inServer:      inServer,
					defaultClient: defaultOutClientForThis,
				}

				go handshakeInserver_and_passToOutClient(iics)
			}

		}()

		if utils.CanLogInfo() {
			log.Println(proxy.GetFullName(inServer), "is listening ", inServer.Network(), "on", inServer.AddrStr())

		}
		return
	}

	handleFunc := func(conn net.Conn) {
		handleNewIncomeConnection(inServer, defaultOutClientForThis, conn)
	}

	network := inServer.Network()
	err := netLayer.ListenAndAccept(network, inServer.AddrStr(), handleFunc)

	if err == nil {
		if utils.CanLogInfo() {
			log.Println(proxy.GetFullName(inServer), "is listening ", network, "on", inServer.AddrStr())

		}

	} else {
		if err != nil {
			log.Fatalln("can not listen inServer on", inServer.AddrStr(), err)

		}
	}
}

type incomingInserverConnState struct {

	//baseLocalConn 是来自客户端的原始网络层链接

	//wrappedConn是层层握手后包装的链接;

	// 在多路复用的情况下, 可能产生多个 IncomingInserverConnState，
	// 共用一个 baseLocalConn, 但是 wrappedConn 各不相同。

	//这里说的多路复用基本指的就是grpc/quic; 如果是 vless内嵌 mux.cool 的话不属于这种情况.

	// 要区分 多路复用的包装 是在 vless等代理的握手验证 的外部 还是 内部

	baseLocalConn, wrappedConn net.Conn
	inServer                   proxy.Server
	defaultClient              proxy.Client

	cachedRemoteAddr string
	theRequestPath   string

	inServerTlsConn            *tlsLayer.Conn
	inServerTlsRawReadRecorder *tlsLayer.Recorder

	shouldFallback bool

	theFallbackFirstBuffer *bytes.Buffer

	isTlsLazyServerEnd bool

	shouldCloseInSerBaseConnWhenFinish bool

	routedToDirect bool
}

// handleNewIncomeConnection 会处理 网络层至高级层的数据，
// 然后将代理层的处理发往 handshakeInserver_and_passToOutClient 函数。
func handleNewIncomeConnection(inServer proxy.Server, defaultClientForThis proxy.Client, thisLocalConnectionInstance net.Conn) {

	//log.Println("handleNewIncomeConnection called", defaultClientForThis)

	iics := incomingInserverConnState{
		baseLocalConn: thisLocalConnectionInstance,
		inServer:      inServer,
		defaultClient: defaultClientForThis,
	}

	iics.isTlsLazyServerEnd = tls_lazy_encrypt && canLazyEncryptServer(inServer)

	wrappedConn := thisLocalConnectionInstance

	if utils.CanLogInfo() {
		str := wrappedConn.RemoteAddr().String()
		log.Println("New Accepted Conn from", str, ", being handled by "+proxy.GetVSI_url(inServer))

		iics.cachedRemoteAddr = str
	}

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

			if utils.CanLogErr() {
				log.Println("failed in inServer tls handshake ", inServer.AddrStr(), err)

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

	//log.Println("handshake passed tls")

	if adv := inServer.AdvancedLayer(); adv != "" {
		switch adv {
		//quic虽然也是adv层，但是因为根本没调用 handleNewIncomeConnection 函数，所以不在此处理
		//

		case "grpc":
			//grpc不太一样, 它是多路复用的
			// 每一条建立好的 grpc 可以随时产生对新目标的请求,

			// 我们直接循环监听然后分别用 新goroutine发向 handshakeInserver_and_passToOutClient

			if utils.CanLogDebug() {
				log.Println("start upgrade grpc")
			}

			grpcs := inServer.GetGRPC_Server() //这个grpc server是在配置阶段初始化好的.

			grpcs.StartHandle(wrappedConn)

			//start之后，客户端就会利用同一条tcp链接 来发送多个 请求,自此就不能直接用原来的链接了;
			// 新的子请求被 grpc包 抽象成了 抽象的 conn
			//遇到chan被关闭的情况后，就会自动关闭底层连接并退出整个函数。
			for {
				newGConn, ok := <-grpcs.NewConnChan
				if !ok {
					if utils.CanLogWarn() {
						log.Println("grpc getNewSubConn not ok")
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
					if utils.CanLogErr() {
						log.Println("ws: got not http request ", inServer.AddrStr())
					}

				} else {
					if utils.CanLogErr() {
						log.Println("ws: handshake read error ", inServer.AddrStr())
					}
				}

				wrappedConn.Close()
				return
			}

			wss := inServer.GetWS_Server()

			if rp.Method != "GET" || wss.Thepath != rp.Path {
				iics.theRequestPath = rp.Path
				iics.theFallbackFirstBuffer = rp.WholeRequestBuf

				if utils.CanLogDebug() {
					log.Println("ws path not match", rp.Method, rp.Path, "should be:", wss.Thepath)

				}

				iics.shouldFallback = true
				goto startPass

			}

			//此时path和method都已经匹配了, 如果还不能通过那就说明后面的header等数据不满足ws的upgrade请求格式, 肯定是非法数据了,也不用再回落
			wsConn, err := wss.Handshake(rp.WholeRequestBuf, wrappedConn)
			if err != nil {
				if utils.CanLogErr() {
					log.Println("failed in inServer websocket handshake ", inServer.AddrStr(), err)

				}

				wrappedConn.Close()
				return

			}
			wrappedConn = wsConn
		} // switch adv

	} //if adv !=""

startPass:

	iics.wrappedConn = wrappedConn

	handshakeInserver_and_passToOutClient(iics)
}

// 本函数 处理inServer的代理层数据，并在试图处理 分流和回落后，将流量导向目标，并开始Copy。
// iics 不使用指针, 因为iics不能公用，因为 在多路复用时 iics.wrappedConn 是会变化的。
func handshakeInserver_and_passToOutClient(iics incomingInserverConnState) {
	//log.Println("handshakeInserver_and_passToOutClient")

	wrappedConn := iics.wrappedConn

	inServer := iics.inServer

	var wlc io.ReadWriteCloser
	var targetAddr *netLayer.Addr
	var err error

	if iics.shouldFallback {
		goto checkFallback
	}

	wlc, targetAddr, err = inServer.Handshake(wrappedConn)
	if err == nil {
		//log.Println("inServer handshake passed")
		//无错误时直接跳过回落, 直接执行下一个步骤
		goto afterLocalServerHandshake
	}

	////////////////////////////// 回落阶段 /////////////////////////////////////

	//下面代码查看是否支持fallback; wlc先设为nil, 当所有fallback都不满足时,可以判断nil然后关闭连接

	wlc = nil

	if utils.CanLogWarn() {
		log.Println("failed in inServer proxy handshake from", inServer.AddrStr(), err)
	}

	if !inServer.CanFallback() {
		wrappedConn.Close()
		return
	}

	{ //为防止goto 跳过变量定义，使用单独代码块

		fe, ok := err.(*utils.ErrFirstBuffer)
		if !ok {
			// 能fallback 但是返回的 err却不是fallback err，证明遇到了更大问题，可能是底层read问题，所以也不用继续fallback了
			wrappedConn.Close()
			return
		}

		iics.theFallbackFirstBuffer = fe.First
		if iics.theFallbackFirstBuffer == nil {
			//不应该，至少能读到1字节的。
			log.Fatal("No FirstBuffer")
		}
	}

checkFallback:

	//先检查 mainFallback，如果mainFallback中各项都不满足 或者根本没有 mainFallback 再检查 defaultFallback

	if mainFallback != nil {

		if utils.CanLogDebug() {
			log.Println("checkFallback")
		}

		var thisFallbackType byte

		fallback_params := make([]string, 0, 4)

		theRequestPath := iics.theRequestPath

		if iics.theFallbackFirstBuffer != nil && theRequestPath == "" {
			var failreason int

			_, theRequestPath, failreason = httpLayer.GetRequestMethod_and_PATH_from_Bytes(iics.theFallbackFirstBuffer.Bytes(), false)

			if failreason != 0 {
				theRequestPath = ""
			}

		}

		if theRequestPath != "" {
			fallback_params = append(fallback_params, theRequestPath)
			thisFallbackType |= httpLayer.Fallback_path
		}

		if inServerTlsConn := iics.inServerTlsConn; inServerTlsConn != nil {
			//默认似乎默认tls不会给出alpn和sni项？获得的是空值,也许是因为我用了自签名+insecure,所以导致server并不会设置连接好后所协商的ServerName
			// 而alpn则也是正常的, 不设置肯定就是空值
			// TODO: 配置中加一个 alpn选项.
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

		fbAddr := mainFallback.GetFallback(thisFallbackType, fallback_params...)

		if utils.CanLogDebug() {
			log.Println("checkFallback ,matched fallback:", fbAddr)
		}
		if fbAddr != nil {
			targetAddr = fbAddr
			wlc = wrappedConn
			goto afterLocalServerHandshake
		}

	}

	//默认回落, 每个listen配置 都可 有一个自己独享的默认回落

	if defaultFallbackAddr := inServer.GetFallback(); defaultFallbackAddr != nil {

		targetAddr = defaultFallbackAddr
		wlc = wrappedConn

	}

afterLocalServerHandshake:

	if wlc == nil {
		//无wlc证明 inServer 握手失败，且 没有任何回落可用, 直接return
		if utils.CanLogDebug() {
			log.Println("invalid request and no matched fallback, hung up.")
		}
		wrappedConn.Close()
		return
	}

	////////////////////////////// 分流阶段 /////////////////////////////////////

	var client proxy.Client = iics.defaultClient

	//尝试分流, 获取到真正要发向 的 outClient
	if routePolicy != nil && !inServer.CantRoute() {

		desc := &netLayer.TargetDescription{
			Addr: targetAddr,
			Tag:  inServer.GetTag(),
		}

		if utils.CanLogDebug() {
			log.Println("try routing", desc)
		}

		outtag := routePolicy.GetOutTag(desc)

		if outtag == "direct" {
			client = directClient
			iics.routedToDirect = true

			if utils.CanLogInfo() {
				log.Println("routed to direct", targetAddr.UrlString())
			}
		} else {
			//log.Println("outtag", outtag, clientsTagMap)

			if tagC, ok := clientsTagMap[outtag]; ok {
				client = tagC
				if utils.CanLogInfo() {
					log.Println("routed to", outtag, proxy.GetFullName(client))
				}
			}
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
		isTlsLazy_clientEnd = tls_lazy_encrypt && canLazyEncryptClient(client)

	}

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个 就是要发送到 outClient 的信息，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5 对 tls 进行判断
	//
	//  而在服务端探测时，因为 客户端传来的连接 包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 inServer 判断 wlc; 总之，含义就是，去检索“用户承载数据”的来源

	if isTlsLazy_clientEnd || iics.isTlsLazyServerEnd {

		if tlsLayer.PDD {
			log.Println("loading TLS SniffConn", isTlsLazy_clientEnd, iics.isTlsLazyServerEnd)
		}

		wlc = tlsLayer.NewSniffConn(iics.baseLocalConn, wlc, isTlsLazy_clientEnd, tls_lazy_secure)

	}

	//这一段代码是去判断是否要在转发结束后自动关闭连接
	//如果目标是udp则要分情况讨论
	//
	// 这里 因为 vless v1 的 CRUMFURS 信道 会对 wrappedConn 进行 keep alive ，
	// 而且具体的传递信息的部分并不是在main函数中处理，而是自己的go routine，所以不能直接关闭 wrappedConn
	// 所以要分情况进行 defer wrappedConn.Close()。
	// 然后这里是设置 iics.shouldCloseBaseConnWhenComplete, 然后在dialClient函数里再实际 defer

	if targetAddr.IsUDP() {

		switch inServer.Name() {
		case "vless":

			if targetAddr.Name == vless.CRUMFURS_Established_Str {
				// 预留了 CRUMFURS 信道的话，就不要关闭 wrappedConn
				// 	而且也不在这里处理监听事件，client自己会在额外的 goroutine里处理
				//	server也一样，会在特定的场合给 CRUMFURS 传值，这个机制是与main函数无关的

				// 而且 wrappedConn 会被 inServer 保存起来，用于后面的 unknownRemoteAddrMsgWriter
				return
			} else {
				//如果不是CRUMFURS命令，那就是普通的针对某udp地址的连接，见下文 uniExtractor 的使用

				iics.shouldCloseInSerBaseConnWhenFinish = true
			}

		case "socks5":
			// UDP Associate：
			// 因为socks5的 UDP Associate 办法是较为特殊的，不使用现有tcp而是新建立udp，所以此时该tcp连接已经没用了
			// 另外，此时 targetAddr.IsUDP 只是用于告知此链接是udp Associate，并不包含实际地址信息
			//但是根据socks5标准，这个tcp链接同样是 keep alive的，否则客户端就会认为服务端挂掉了.
		default:
			iics.shouldCloseInSerBaseConnWhenFinish = true

		}
	} else {

		//lazy_encrypt情况比较特殊，基础连接何时被关闭会在tlslazy相关代码中处理。
		// 如果不是lazy的情况的话，转发结束后，要自动关闭
		if !iics.isTlsLazyServerEnd {

			//实测 grpc.Conn 被调用了Close 也不会实际关闭连接，而是会卡住，阻塞，直到底层tcp连接被关闭后才会返回
			// 但是我们还是 直接避免这种情况
			if !inServer.IsMux() {
				iics.shouldCloseInSerBaseConnWhenFinish = true

			}

		}

	}

	// 下面一段代码 单独处理 udp承载数据的特殊转发。
	//
	// 这里只处理 vless v1 的CRUMFURS  转发到direct的情况 以及 socks5 的udp associate 转发到vless 的情况;
	// 如果条件不符合则会跳过而进入下一阶段
	if targetAddr.IsUDP() {

		switch inServer.Name() {
		case "vless":

			if client.Name() == "direct" {

				uc := wlc.(*vless.UserConn)

				if uc.GetProtocolVersion() < 1 {
					break
				}

				// 根据 vless_v1的讨论，vless_v1 的udp转发的 通信方式 也是与tcp连接类似的分离信道方式
				//	上面已经把 CRUMFURS 的情况过滤掉了，所以现在这里就是普通的udp请求
				//
				// 因为direct使用 proxy.RelayUDP_to_Direct 函数 直接实现了fullcone
				// 那么我们只需要传入一个  UDP_Extractor 即可

				//unknownRemoteAddrMsgWriter 在 vless v1中的实现就是 theCRUMFURS （vless v0就是mux）

				id := uc.GetIdentityStr()

				vlessServer := inServer.(*vless.Server)

				theCRUMFURS := vlessServer.Get_CRUMFURS(id)
				var unknownRemoteAddrMsgWriter netLayer.UDPResponseWriter

				unknownRemoteAddrMsgWriter = theCRUMFURS

				uniExtractor := netLayer.NewUniUDP_Extractor(targetAddr.ToUDPAddr(), wlc, unknownRemoteAddrMsgWriter)

				netLayer.RelayUDP_to_Direct(uniExtractor) //阻塞

				return
			}

		case "socks5":
			// 此时socks5包已经帮我们dial好了一个udp连接，即wlc，但是还未读取到客户端想要访问的东西
			udpConn := wlc.(*socks5.UDPConn)

			dialFunc := func(targetAddr *netLayer.Addr) (io.ReadWriter, error) {
				return dialClient(incomingInserverConnState{}, targetAddr, client, false, nil, true)
			}

			// 将 outClient 视为 UDP_Putter ，就可以转发udp信息了

			//direct 和 vless 的Client 都实现了 UDP_Putter.

			// direct 通过 UDP_Pipe和 RelayUDP_to_Direct函数 实现了 UDP_Putter

			// vless 的client 实现了 UDP_Putter, 新连接的Handshake过程会在 dialFunc 被调用 时发生

			if putter := client.(netLayer.UDP_Putter); putter != nil {

				//UDP_Putter 不使用传统的Handshake过程，因为Handshake是用于第一次数据，然后后面接着的双向传输都不再需要额外信息；而 UDP_Putter 每一次数据传输都是需要传输 目标地址的，所以每一次都需要一些额外数据，这就是我们 UDP_Putter 接口去解决的事情。

				//因为UDP Associate后，就会保证以后的向 wlc 的 所有请求数据都是udp请求，所以可以在这里直接循环转发了。

				go udpConn.StartPushResponse(putter)

				udpConn.StartReadRequest(putter, dialFunc)

			} else {
				if utils.CanLogErr() {
					log.Println("socks5 server -> client for udp, but client didn't implement netLayer.UDP_Putter", client.Name())
				}
			}
			return

		}

	}

	////////////////////////////// 拨号阶段 /////////////////////////////////////

	//log.Println("will dial", client)
	dialClient(iics, targetAddr, client, isTlsLazy_clientEnd, wlc, false)
}

// dialClient 对实际client进行拨号，处理传输层, tls层, 高级层等所有层级后，进行代理层握手，
// 然后 进行实际转发(Copy)。
// targetAddr为用户所请求的地址。
//client为真实要拨号的client，可能会与iics里的defaultClient不同。以client为准。
// wlc为调用者所提供的 此请求的 来源 链接。wlc主要用于 Copy阶段.
// noCopy是为了让其它调用者自行处理 转发 时使用。
func dialClient(iics incomingInserverConnState, targetAddr *netLayer.Addr, client proxy.Client, isTlsLazy_clientEnd bool, wlc io.ReadWriteCloser, noCopy bool) (io.ReadWriter, error) {

	if iics.shouldCloseInSerBaseConnWhenFinish && !noCopy {
		if iics.baseLocalConn != nil {
			defer iics.baseLocalConn.Close()
		}
	}

	var err error

	//先确认拨号地址

	//direct的话自己是没有目的地址的，直接使用 请求的地址
	// 而其它代理的话, realTargetAddr会被设成实际配置的代理的地址
	var realTargetAddr *netLayer.Addr = targetAddr

	if uniqueTestDomain != "" && uniqueTestDomain != targetAddr.Name {
		if utils.CanLogDebug() {
			log.Println("request isn't the appointed domain", targetAddr, uniqueTestDomain)

		}
		return nil, utils.NumErr{N: 1, Prefix: "dialClient err, "}
	}

	if utils.CanLogInfo() {
		log.Println(iics.cachedRemoteAddr, " request ", targetAddr.UrlString(), "through", proxy.GetVSI_url(client))

	}

	if client.AddrStr() != "" {
		//log.Println("will dial", client.AddrStr())

		realTargetAddr, err = netLayer.NewAddr(client.AddrStr())
		if err != nil {
			log.Fatal("convert addr err:", err)
		}
		realTargetAddr.Network = client.Network()
	}
	var clientEndRemoteClientTlsRawReadRecorder *tlsLayer.Recorder

	var clientConn net.Conn

	var grpcClientConn grpc.ClientConn
	var dailedCommonConn any

	//如果是单路的, 则我们在此dial, 如果是多路复用, 则不行, 因为要复用同一个连接
	// Instead, 我们要试图从grpc中取出已经拨号好了的 grpc链接

	switch client.AdvancedLayer() {
	case "grpc":

		grpcClientConn = grpc.GetEstablishedConnFor(realTargetAddr)

		if grpcClientConn != nil {
			//如果有已经建立好的连接，则跳过拨号阶段
			goto advLayerStep
		}
	case "quic":
		//quic这里并不是在tls层基础上进行dial，而是直接dial
		dailedCommonConn = client.DialCommonInitialLayerConnFunc()(realTargetAddr)
		if dailedCommonConn != nil {
			//如果有已经建立好的连接，则跳过拨号阶段
			goto advLayerStep
		} else {
			//dail失败, 直接return掉

			return nil, utils.NumErr{N: 13, Prefix: "dial quic Client err"}
		}
	}

	clientConn, err = realTargetAddr.Dial()
	if err != nil {
		if utils.CanLogErr() {
			log.Println("failed in dial", realTargetAddr.String(), ", Reason: ", err)

		}
		return nil, utils.NumErr{N: 2, Prefix: "dialClient err, "}
	}

	//log.Println("dial real addr ok", realTargetAddr)

	////////////////////////////// tls握手阶段 /////////////////////////////////////

	if client.IsUseTLS() { //即客户端

		if isTlsLazy_clientEnd {

			if tls_lazy_secure && wlc != nil {
				// 如果使用secure办法，则我们每次不能先拨号，而是要detect用户的首包后再拨号
				// 这种情况只需要客户端操作, 此时我们wrc直接传入原始的 刚拨号好的 tcp连接，即 clientConn

				// 而且为了避免黑客攻击或探测，我们要使用uuid作为特殊指令，此时需要 UserServer和 UserClient

				if uc := client.(proxy.UserClient); uc != nil {
					tryTlsLazyRawCopy(true, uc, nil, targetAddr, clientConn, wlc, nil, true, nil)

				}

				return nil, utils.NumErr{N: 3, Prefix: "dialClient err, "}

			} else {
				clientEndRemoteClientTlsRawReadRecorder = tlsLayer.NewRecorder()
				teeConn := tlsLayer.NewTeeConn(clientConn, clientEndRemoteClientTlsRawReadRecorder)

				clientConn = teeConn
			}
		}

		tlsConn, err := client.GetTLS_Client().Handshake(clientConn)
		if err != nil {
			log.Println("failed in handshake outClient tls", targetAddr.String(), ", Reason: ", err)
			return nil, utils.NumErr{N: 4, Prefix: "dialClient err, "}
		}

		clientConn = tlsConn

	}

	////////////////////////////// 高级层握手阶段 /////////////////////////////////////

advLayerStep:

	if adv := client.AdvancedLayer(); adv != "" {
		switch adv {
		case "quic":
			clientConn, err = client.DialSubConnFunc()(dailedCommonConn)
			if err != nil {
				if utils.CanLogErr() {
					log.Println("DialSubConnFunc failed,", err)
				}
				return nil, utils.NumErr{N: 14, Prefix: "DialSubConnFunc err, "}
			}
		case "grpc":
			if grpcClientConn == nil {
				grpcClientConn, err = grpc.ClientHandshake(clientConn, realTargetAddr)
				if err != nil {
					if utils.CanLogErr() {
						log.Println("grpc.ClientHandshake failed,", err)

					}
					if iics.baseLocalConn != nil {
						iics.baseLocalConn.Close()

					}
					return nil, utils.NumErr{N: 5, Prefix: "dialClient err, "}
				}

			}

			clientConn, err = grpc.DialNewSubConn(client.Path(), grpcClientConn, realTargetAddr)
			if err != nil {
				if utils.CanLogErr() {
					log.Println("grpc.DialNewSubConn failed,", err)

					//如果底层tcp连接被关闭了的话，错误会是：
					// rpc error: code = Unavailable desc = connection error: desc = "transport: failed to write client preface: tls: use of closed connection"

				}
				if iics.baseLocalConn != nil {
					iics.baseLocalConn.Close()
				}
				return nil, utils.NumErr{N: 6, Prefix: "dialClient err, "}
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
					if utils.CanLogErr() {
						log.Println("err when reading ws early data", e)
					}
					return nil, utils.NumErr{N: 7, Prefix: "dialClient err, "}
				}
				ed = edBuf[:n]
				//log.Println("will send early data", n, ed)

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
			//wc, err := wsClient.Handshake(clientConn, ed)
			if err != nil {
				if utils.CanLogErr() {
					log.Println("failed in handshake ws to", targetAddr.String(), ", Reason: ", err)

				}
				return nil, utils.NumErr{N: 8, Prefix: "dialClient err, "}
			}

			clientConn = wc
		}
	}

	////////////////////////////// 代理层 握手阶段 /////////////////////////////////////

	wrc, err := client.Handshake(clientConn, targetAddr)
	if err != nil {
		if utils.CanLogErr() {
			log.Println("failed in handshake to", targetAddr.String(), ", Reason: ", err)

		}
		return nil, utils.NumErr{N: 9, Prefix: "dialClient err, "}
	}
	//log.Println("all handshake finished")

	////////////////////////////// 实际转发阶段 /////////////////////////////////////

	if noCopy {
		return wrc, nil
	}

	if !iics.routedToDirect && tls_lazy_encrypt {

		// 我们加了回落之后，就无法确定 “未使用tls的outClient 一定是在服务端” 了
		if isTlsLazy_clientEnd {

			if client.IsUseTLS() {
				//必须是 UserClient
				if userClient := client.(proxy.UserClient); userClient != nil {
					tryTlsLazyRawCopy(false, userClient, nil, nil, wrc, wlc, iics.baseLocalConn, true, clientEndRemoteClientTlsRawReadRecorder)
					return nil, utils.NumErr{N: 11, Prefix: "dialClient err, "}
				}
			}

		} else if iics.isTlsLazyServerEnd {

			// 最新代码已经确认，使用uuid 作为 “特殊指令”，所以要求Server必须是一个 proxy.UserServer
			// 否则将无法开启splice功能。这是为了防止0-rtt 探测;

			if userServer, ok := iics.inServer.(proxy.UserServer); ok {
				tryTlsLazyRawCopy(false, nil, userServer, nil, wrc, wlc, iics.baseLocalConn, false, iics.inServerTlsRawReadRecorder)
				return nil, utils.NumErr{N: 12, Prefix: "dialClient err, "}
			}

		}

	}

	if iics.theFallbackFirstBuffer != nil {
		//这里注意，因为是把 tls解密了之后的数据发送到目标地址，所以这种方式只支持转发到本机纯http服务器
		wrc.Write(iics.theFallbackFirstBuffer.Bytes())
		utils.PutBytes(iics.theFallbackFirstBuffer.Bytes()) //这个Buf不是从utils.GetBuf创建的，而是从一个 GetBytes的[]byte 包装 的，所以我们要PutBytes，而不是PutBuf
	}

	if utils.CanLogDebug() {

		netLayer.DebugRelay(realTargetAddr, wrc, wlc)

	} else {
		netLayer.Relay(wlc, wrc)
	}

	return wrc, nil
}
