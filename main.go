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
	"runtime"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/grpc"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
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
const tlslazy_willuseSystemCall = runtime.GOOS == "linux" || runtime.GOOS == "darwin"

var (
	configFileName string

	uniqueTestDomain string //有时需要测试到单一网站的流量，此时为了避免其它干扰，需要在这里声明 一下 该域名，然后程序里会进行过滤

	confMode         int = -1 //0: simple json, 1: standard toml, 2: v2ray compatible json
	simpleConf       *proxy.Simple
	standardConf     *proxy.Standard
	directClient     proxy.Client
	defaultOutClient proxy.Client
	defaultInServer  proxy.Server
	default_uuid     string

	allServers = make([]proxy.Server, 0, 8)
	allClients = make([]proxy.Client, 0, 8)

	serversTagMap = make(map[string]proxy.Server)
	clientsTagMap = make(map[string]proxy.Client)

	listenURL string
	dialURL   string

	tls_lazy_encrypt bool
	tls_lazy_secure  bool

	routePolicy  *netLayer.RoutePolicy
	mainFallback *httpLayer.ClassicFallback

	isServerEnd bool //这个是代码里推断的，不一定准确；不过目前仅被用于tls lazy encrypt，所以不是很重要

	cmdPrintSupportedProtocols bool
)

func init() {
	directClient, _ = proxy.ClientFromURL("direct://")
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")

	flag.BoolVar(&tls_lazy_encrypt, "lazy", false, "tls lazy encrypt (splice)")
	flag.BoolVar(&tls_lazy_secure, "ls", false, "tls lazy secure, use special techs to ensure the tls lazy encrypt data can't be detected. Only valid at client end.")

	flag.StringVar(&configFileName, "c", "client.toml", "config file name")

	flag.StringVar(&listenURL, "L", "", "listen URL (i.e. the local part in config file), only enbled when config file is not provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL (i.e. the remote part in config file), only enbled when config file is not provided.")

	flag.StringVar(&uniqueTestDomain, "td", "", "test a single domain, like www.domain.com")

}

func mayPrintSupportedProtocols() {

	if !cmdPrintSupportedProtocols {
		return
	}
	proxy.PrintAllServerNames()

	proxy.PrintAllClientNames()
}

func main() {

	printVersion()

	flag.Parse()

	mayPrintSupportedProtocols()

	cmdLL := utils.LogLevel
	cmdUseReadv := netLayer.UseReadv
	loadConfig()

	if confMode < 0 {
		log.Fatal("no config exist")
	}

	//有点尴尬, 读取配置文件必须要用命令行参数，而配置文件里的部分配置又会覆盖部分命令行参数

	if cmdLL != utils.DefaultLL && utils.LogLevel != cmdLL {
		//配置文件配置了日志等级, 但是因为 命令行给出的值优先, 所以要设回

		utils.LogLevel = cmdLL
	}

	if cmdUseReadv == false && netLayer.UseReadv == true {
		//配置文件配置了日志等级, 但是因为 命令行给出的值优先, 所以要设回

		netLayer.UseReadv = false
	}

	fmt.Println("Log Level:", utils.LogLevel)
	fmt.Println("UseReadv:", netLayer.UseReadv)

	var err error

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

	isServerEnd = defaultOutClient.Name() == "direct"

	// 后台运行主代码，而main函数只监听中断信号
	// TODO: 未来main函数可以推出 交互模式，等未来推出动态增删用户、查询流量等功能时就有用;
	//  或可用于交互生成自己想要的配置
	if confMode == simpleMode {
		listenSer(nil, defaultInServer)
	} else {
		for _, inServer := range allServers {
			listenSer(nil, inServer)
		}
	}

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}

//非阻塞
func listenSer(listener net.Listener, inServer proxy.Server) {

	handleFunc := func(conn net.Conn) {
		handleNewIncomeConnection(inServer, conn)
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

func handleNewIncomeConnection(inServer proxy.Server, thisLocalConnectionInstance net.Conn) {

	baseLocalConn := thisLocalConnectionInstance

	var cachedRemoteAddr string

	if utils.CanLogInfo() {
		cachedRemoteAddr = thisLocalConnectionInstance.RemoteAddr().String()
		log.Println("New req from", cachedRemoteAddr)

	}

	//此时，baseLocalConn里面 正常情况下, 服务端看到的是 客户端的golang的tls 拨号发出的 tls数据
	// 客户端看到的是 socks5的数据， 我们首先就是要看看socks5里的数据是不是tls，而socks5自然 IsUseTLS 是false

	// 如果是服务端的话，那就是 inServer.IsUseTLS == true, 此时，我们正常握手，然后我们需要判断的是它承载的数据

	// 每次tls试图从 原始连接 读取内容时，都会附带把原始数据写入到 这个 Recorder中
	var inServerTlsRawReadRecorder *tlsLayer.Recorder

	var inServerTlsConn *tlsLayer.Conn

	if inServer.IsUseTLS() {

		if tls_lazy_encrypt {
			inServerTlsRawReadRecorder = tlsLayer.NewRecorder()

			inServerTlsRawReadRecorder.StopRecord() //先不记录，因为一开始是我们自己的tls握手包，没有意义
			teeConn := tlsLayer.NewTeeConn(baseLocalConn, inServerTlsRawReadRecorder)

			thisLocalConnectionInstance = teeConn
		}

		tlsConn, err := inServer.GetTLS_Server().Handshake(thisLocalConnectionInstance)
		if err != nil {

			if utils.CanLogErr() {
				log.Println("failed in inServer tls handshake ", inServer.AddrStr(), err)

			}
			thisLocalConnectionInstance.Close()
			return
		}

		if tls_lazy_encrypt {
			//此时已经握手完毕，可以记录了
			inServerTlsRawReadRecorder.StartRecord()
		}

		inServerTlsConn = tlsConn
		thisLocalConnectionInstance = tlsConn

	}

	var theFallbackFirstBuffer *bytes.Buffer

	var theRequestPath string

	var shouldFallback bool

	//var defaultFallback httpLayer.Fallback

	//log.Println("handshake passed tls")

	if adv := inServer.AdvancedLayer(); adv != "" {
		switch adv {
		case "grpc":
			//grpc不太一样, 它是多路复用的
			// 每一条建立好的 grpc 可以随时产生对新目标的请求

			grpcs := inServer.GetGRPC_Server() //这个grpc server是在配置阶段初始化好的.
			grpc.HandleRawConn(grpcs, "", thisLocalConnectionInstance)

		case "ws":

			//从ws开始就可以应用fallback了
			// 但是，不能先upgrade, 因为我们要用path分流, 所以我们要先预读;
			// 否则的话，正常的http流量频繁地被用不到的 ws 过滤器处理,会损失很多性能,而且gobwas没办法简洁地保留初始buffer.

			var rp httpLayer.RequestParser
			re := rp.ReadAndParse(thisLocalConnectionInstance)
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

				thisLocalConnectionInstance.Close()
				return
			}

			wss := inServer.GetWS_Server()

			if rp.Method != "GET" || wss.Thepath != rp.Path {
				theRequestPath = rp.Path
				theFallbackFirstBuffer = rp.WholeRequestBuf

				if utils.CanLogDebug() {
					log.Println("ws path not match", rp.Method, rp.Path, "should be:", wss.Thepath)

				}

				shouldFallback = true
				goto startPass

			}

			//此时path和method都已经匹配了, 如果还不能通过那就说明后面的header等数据不满足ws的upgrade请求格式, 肯定是非法数据了,也不用再回落
			wsConn, err := wss.Handshake(rp.WholeRequestBuf, thisLocalConnectionInstance)
			if err != nil {
				if utils.CanLogErr() {
					log.Println("failed in inServer websocket handshake ", inServer.AddrStr(), err)

				}

				thisLocalConnectionInstance.Close()
				return

			}
			thisLocalConnectionInstance = wsConn
		} // switch adv

	} //if adv !=""

startPass:

	handshakeInserver_and_passToOutClient(baseLocalConn, thisLocalConnectionInstance, inServerTlsConn, inServer, shouldFallback, theFallbackFirstBuffer, theRequestPath, inServerTlsRawReadRecorder, cachedRemoteAddr)
}

func handshakeInserver_and_passToOutClient(baseLocalConn, thisLocalConnectionInstance net.Conn, inServerTlsConn *tlsLayer.Conn, inServer proxy.Server, shouldFallback bool, theFallbackFirstBuffer *bytes.Buffer, theRequestPath string, inServerTlsRawReadRecorder *tlsLayer.Recorder, cachedRemoteAddr string) {

	defaultFallbackAddr := inServer.GetFallback()
	var wlc io.ReadWriter
	var targetAddr *netLayer.Addr
	var err error

	if shouldFallback {
		goto checkFallback
	}

	wlc, targetAddr, err = inServer.Handshake(thisLocalConnectionInstance)
	if err == nil {
		//无错误时直接进行下一步了
		goto afterLocalServerHandshake
	}

	//下面代码查看是否支持fallback; wlc先设为nil, 当所有fallback都不满足时,可以判断nil然后关闭连接

	wlc = nil

	if utils.CanLogWarn() {
		log.Println("failed in inServer proxy handshake from", inServer.AddrStr(), err)
	}

	if !inServer.CanFallback() {
		thisLocalConnectionInstance.Close()
		return
	}

	{ //为防止goto 跳过变量定义，使用单独代码块

		fe, ok := err.(*utils.ErrFirstBuffer)
		if !ok {
			// 能fallback 但是返回的 err却不是fallback err，证明遇到了更大问题，可能是底层read问题，所以也不用继续fallback了
			thisLocalConnectionInstance.Close()
			return
		}

		theFallbackFirstBuffer = fe.First
		if theFallbackFirstBuffer == nil {
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

		if theFallbackFirstBuffer != nil && theRequestPath == "" {
			var failreason int

			_, theRequestPath, failreason = httpLayer.GetRequestMethod_and_PATH_from_Bytes(theFallbackFirstBuffer.Bytes(), false)

			if failreason != 0 {
				theRequestPath = ""
			}

		}

		if theRequestPath != "" {
			fallback_params = append(fallback_params, theRequestPath)
			thisFallbackType |= httpLayer.Fallback_path
		}

		if inServerTlsConn != nil {
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
			wlc = thisLocalConnectionInstance
			goto afterLocalServerHandshake
		}

	}

	//默认回落

	if defaultFallbackAddr != nil {

		targetAddr = defaultFallbackAddr
		wlc = thisLocalConnectionInstance

	}

afterLocalServerHandshake:

	if wlc == nil {
		//回落也失败, 直接return
		if utils.CanLogDebug() {
			log.Println("invalid request and no matched fallback, hung up.")
		}
		thisLocalConnectionInstance.Close()
		return
	}

	var client proxy.Client = defaultOutClient

	var routedToDirect bool

	//尝试分流
	if !inServer.CantRoute() && routePolicy != nil {

		if utils.CanLogDebug() {
			log.Println("try routing")
		}

		outtag := routePolicy.GetOutTag(&netLayer.TargetDescription{
			Addr: targetAddr,
			Tag:  inServer.GetTag(),
		})
		if outtag == "direct" {
			client = directClient
			routedToDirect = true

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

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个 就是要发送到 outClient 的信息，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5 对 tls 进行判断
	//
	//  而在服务端探测时，因为 客户端传来的连接 包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 inServer 判断 wlc; 总之，含义就是，去检索“用户承载数据”的来源

	if tls_lazy_encrypt && !(!isServerEnd && routedToDirect) {

		if tlsLayer.PDD {
			log.Println("loading TLS SniffConn", !isServerEnd)
		}

		wlc = tlsLayer.NewSniffConn(baseLocalConn, wlc, !isServerEnd, tls_lazy_secure)
	}

	//如果目标是udp则要分情况讨论
	if targetAddr.Network == "udp" {

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

			if putter := client.(netLayer.UDP_Putter); putter != nil {

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

	if targetAddr.Network == "udp" {

		var unknownRemoteAddrMsgWriter netLayer.UDPResponseWriter

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

		uniExtractor := netLayer.NewUniUDP_Extractor(targetAddr.ToUDPAddr(), wlc, unknownRemoteAddrMsgWriter)

		netLayer.RelayUDP_to_Direct(uniExtractor) //阻塞

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
		log.Println(client.Name(), cachedRemoteAddr, " want to dial ", proxy.GetFullName(client), targetAddr.UrlString())

	}

	if client.AddrStr() != "" {
		//log.Println("will dial", client.AddrStr())

		realTargetAddr, err = netLayer.NewAddr(client.AddrStr())
		if err != nil {
			log.Fatal("convert addr err:", err)
		}
		realTargetAddr.Network = client.Network()
	}

	var clientConn net.Conn

	//如果是单路的, 则我们在此dial, 如果是多路复用, 则不行, 因为要复用同一个连接

	if client.IsMux() {

	} else {
		clientConn, err = realTargetAddr.Dial()
		if err != nil {
			if utils.CanLogErr() {
				log.Println("failed in dial", realTargetAddr.String(), ", Reason: ", err)

			}
			return
		}
	}

	//log.Println("dial real addr ok", realTargetAddr)

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

	if adv := client.AdvancedLayer(); adv != "" {
		switch adv {
		case "grpc":
			//grpc虽然是多路复用的, 但是这个已经在 grpc/client.go 里处理了, 我们无需担心, 正常操作即可

		case "ws":
			wsClient := client.GetWS_Client()

			var ed []byte

			if wsClient.UseEarlyData {
				//若配置了 MaxEarlyDataLen，则我们先读一段;
				edBuf := utils.GetPacket()
				edBuf = edBuf[:ws.MaxEarlyDataLen]
				n, e := wlc.Read(edBuf)
				if e != nil {
					if utils.CanLogErr() {
						log.Println("err when reading ws early data", e)
					}
					return
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
				return
			}

			clientConn = wc
		}
	}

	wrc, err := client.Handshake(clientConn, targetAddr)
	if err != nil {
		if utils.CanLogErr() {
			log.Println("failed in handshake to", targetAddr.String(), ", Reason: ", err)

		}
		return
	}
	//log.Println("all handshake finished")

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
				tryRawCopy(false, nil, userServer, nil, wrc, wlc, baseLocalConn, false, inServerTlsRawReadRecorder)
				return
			}

		}

	}

	if theFallbackFirstBuffer != nil {
		//这里注意，因为是把 tls解密了之后的数据发送到目标地址，所以这种方式只支持转发到本机纯http服务器
		wrc.Write(theFallbackFirstBuffer.Bytes())
		utils.PutBytes(theFallbackFirstBuffer.Bytes()) //这个Buf不是从utils.GetBuf创建的，而是从一个 GetBytes的[]byte 包装 的，所以我们要PutBytes，而不是PutBuf
	}

	if utils.CanLogDebug() {

		if netLayer.UseReadv {
			go func() {
				n, e := netLayer.TryCopy(wrc, wlc)
				log.Println("本地->远程 转发结束", realTargetAddr.String(), n, e)
			}()

			n, e := netLayer.TryCopy(wlc, wrc)
			log.Println("远程->本地 转发结束", realTargetAddr.String(), n, e)

		} else {

			go func() {
				n, e := io.Copy(wrc, wlc)
				log.Println("本地->远程 转发结束", realTargetAddr.String(), n, e)
			}()
			n, e := io.Copy(wlc, wrc)

			log.Println("远程->本地 转发结束", realTargetAddr.String(), n, e)

		}

	} else {
		netLayer.Relay(wlc, wrc)
	}

}
