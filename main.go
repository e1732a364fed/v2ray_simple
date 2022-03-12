package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/hahahrfool/v2ray_simple/common"
	"github.com/hahahrfool/v2ray_simple/proxy/direct"
	"github.com/hahahrfool/v2ray_simple/proxy/socks5"
	"github.com/hahahrfool/v2ray_simple/proxy/vless"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

var (
	version = "1.0.0"
	desc    = "v2ray_simple, a very simple implementation of V2Ray, 并且在某些地方试图走在v2ray前面"

	configFileName string

	uniqueTestDomain string //有时需要测试到单一网站的流量，此时为了避免其它干扰，需要在这里声明 一下 该域名，然后程序里会进行过滤

	conf *Config
	//directClient proxy.Client

	tls_lazy_encrypt    bool
	tls_half_lazy       bool
	tls_lazy_fix        bool
	tls_lazy_milisecond int
)

func init() {
	//directClient, _ = proxy.ClientFromURL("direct://")

	flag.BoolVar(&tls_lazy_encrypt, "lazy", false, "tls lazy encrypt (splice)")
	flag.BoolVar(&tls_half_lazy, "hl", false, "tls half lazy, filter data when write and use splice when read; only take effect when '-lazy' is set")
	flag.BoolVar(&tls_lazy_fix, "tlf", false, "tls lazy fix, it tries to sleep 1 second to make sure try to overcome splice problem, but it sacrifice delay. Only used at Server end")

	flag.StringVar(&configFileName, "c", "client.json", "config file name")
	flag.IntVar(&tls_lazy_milisecond, "tlft", 1000, "tls lazy fix delay time,in milisecond.")

	flag.StringVar(&uniqueTestDomain, "td", "", "test a single domain")
}

func printVersion() {
	fmt.Printf("===============================\nv2ray_simple %v (%v), %v %v %v\n", version, desc, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	proxy.PrintAllServerNames()

	proxy.PrintAllClientNames()

	fmt.Printf("=============== 所有协议均可套tls ================\n")
}

type Config struct {
	Server_ThatListenPort_Url string `json:"local"`
	//RouteMethod               string `json:"route"`
	Client_ThatDialRemote_Url string `json:"remote"`
}

func loadConfig(fileName string) (*Config, error) {
	path := common.GetFilePath(fileName)
	if len(path) > 0 {
		if cf, err := os.Open(path); err == nil {
			defer cf.Close()
			bytes, _ := ioutil.ReadAll(cf)
			config := &Config{}
			if err = json.Unmarshal(bytes, config); err != nil {
				return nil, fmt.Errorf("can not parse config file %v, %v", fileName, err)
			}
			return config, nil
		}
	}
	return nil, fmt.Errorf("can not load config file %v", fileName)
}

func main() {

	printVersion()

	flag.Parse()

	var err error

	conf, err = loadConfig(configFileName)
	if err != nil {
		log.Println("can not load config file: ", err)
		os.Exit(-1)
	}

	localServer, err := proxy.ServerFromURL(conf.Server_ThatListenPort_Url)
	if err != nil {
		log.Println("can not create local server: ", err)
		os.Exit(-1)
	}
	defer localServer.Stop()

	remoteClient, err := proxy.ClientFromURL(conf.Client_ThatDialRemote_Url)
	if err != nil {
		log.Println("can not create remote client: ", err)
		os.Exit(-1)
	}

	listener, err := net.Listen("tcp", localServer.AddrStr())
	if err != nil {
		log.Println("can not listen on", localServer.AddrStr(), err)
		os.Exit(-1)
	}
	log.Println(localServer.Name(), "is listening TCP on ", localServer.AddrStr())

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
					time.Sleep(time.Millisecond * 500)
				}
				continue
			}

			go handleNewIncomeConnection(localServer, remoteClient, lc)

		}
	}()

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}

func handleNewIncomeConnection(localServer proxy.Server, remoteClient proxy.Client, thisLocalConnectionInstance net.Conn) {

	baseLocalConn := thisLocalConnectionInstance

	//log.Println("got new", thisLocalConnectionInstance.RemoteAddr().String())

	var err error

	//此时，baseLocalConn里面 正常情况下, 服务端看到的是 客户端的golang的tls 拨号发出的 tls数据
	// 客户端看到的是 socks5的数据， 我们首先就是要看看socks5里的数据是不是tls，而socks5自然 IsUseTLS 是false

	// 如果是服务端的话，那就是 localServer.IsUseTLS == true, 此时，我们正常握手，然后我们需要判断的是它承载的数据

	if localServer.IsUseTLS() {

		tlsConn, err := localServer.GetTLS_Server().Handshake(thisLocalConnectionInstance)
		if err != nil {
			log.Println("failed in handshake localServer tls", localServer.AddrStr(), err)
			thisLocalConnectionInstance.Close()
			return
		}

		thisLocalConnectionInstance = tlsConn

	}

	wlc, targetAddr, err := localServer.Handshake(thisLocalConnectionInstance)
	if err != nil {
		log.Println("failed in handshake from", localServer.AddrStr(), err)
		thisLocalConnectionInstance.Close()
		return
	}

	var client proxy.Client

	client = remoteClient //如果加了白名单等过滤方式，则client可能会等于direct等，再说

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个和要发送到tls的信息是一模一样的，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5进行判断
	//
	//  而在服务端探测时，因为 客户端传来的连接 包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 localServer判断 wlc；总之，含义就是，去检索“用户承载数据”的来源

	if tls_lazy_encrypt {

		wlc = tlsLayer.NewDetectConn(baseLocalConn, wlc, client.IsUseTLS())

		//clientConn = cc
	}

	//如果目标是udp则要分情况讨论
	if targetAddr.IsUDP {

		switch localServer.Name() {
		case "vlesss":
			fallthrough
		case "vless":

			if targetAddr.Name == vless.CRUMFURS_Established_Str {
				// 预留了 CRUMFURS 信道的话，就不要关闭 thisLocalConnectionInstance
				// 	而且也不在这里处理监听事件，client自己会在额外的 goroutine里处理
				//	server也一样，会在特定的场合给 CRUMFURS 传值，这个机制是与main函数无关的

				// 而且 thisLocalConnectionInstance 会被 localServer 保存起来，用于后面的 unknownRemoteAddrMsgWriter

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

			// 将 remoteClient 视为 UDP_Putter ，就可以转发udp信息了
			// direct 也实现了 UDP_Putter (通过 UDP_Pipe和 RelayUDP_to_Direct函数), 所以目前 socks5直接转发udp到direct 的功能 已经实现。

			// 我在 vless v1 的client 的 UserConn 中实现了 UDP_Putter, vless 的 client的 新连接的Handshake过程会在 UDP_Putter.WriteUDPRequest 被调用 时发生

			if putter := remoteClient.(proxy.UDP_Putter); putter != nil {

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

		switch localServer.Name() {
		case "vlesss":
			fallthrough
		case "vless":

			uc := wlc.(*vless.UserConn)

			if uc.GetProtocolVersion() < 1 {
				break
			}

			//unknownRemoteAddrMsgWriter 在 vless v1中的实现就是 theCRUMFURS （vless v0就是mux）

			id := uc.GetIdentityStr()

			vlessServer := localServer.(*vless.Server)

			theCRUMFURS := vlessServer.Get_CRUMFURS(id)

			unknownRemoteAddrMsgWriter = theCRUMFURS
		}

		uniExtractor := proxy.NewUniUDP_Extractor(targetAddr.ToUDPAddr(), wlc, unknownRemoteAddrMsgWriter)

		direct.RelayUDP_to_Direct(uniExtractor) //阻塞

		return
	}

	var realTargetAddr *proxy.Addr = targetAddr //direct的话自己是没有目的地址的，直接使用 请求的地址

	if uniqueTestDomain != "" && uniqueTestDomain != targetAddr.Name {
		log.Println("request not contain same domain", targetAddr, uniqueTestDomain)
		return
	}

	log.Println(client.Name(), " want to dial ", targetAddr.UrlString())

	if client.AddrStr() != "" {
		//log.Println("will dial", client.AddrStr())

		realTargetAddr, _ = proxy.NewAddr(client.AddrStr())
	}

	clientConn, err := realTargetAddr.Dial()
	if err != nil {
		log.Println("failed in dial", targetAddr.String(), ", Reason: ", err)
		return
	}

	if client.IsUseTLS() {

		tlsConn, err := client.GetTLS_Client().Handshake(clientConn)
		if err != nil {
			log.Println("failed in handshake remoteClient tls", targetAddr.String(), ", Reason: ", err)
			return
		}

		clientConn = tlsConn

	}

	wrc, err := client.Handshake(clientConn, targetAddr)
	if err != nil {
		log.Println("failed in handshake to", targetAddr.String(), ", Reason: ", err)
		return
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

	if tls_lazy_encrypt {
		tryRawCopy(wrc, wlc, baseLocalConn, client.IsUseTLS())
		return
	}

	go io.Copy(wrc, wlc)
	io.Copy(wlc, wrc)

}

// tryRawCopy 尝试能否直接对拷，对拷 直接使用 原始 TCPConn
//和 xtls的splice 含义相同
// 我们内部先 使用 DetectConn进行过滤分析，然后再判断进化为splice 或者退化为普通拷贝
func tryRawCopy(wrc, wlc io.ReadWriter, localConn net.Conn, isclient bool) {

	//如果用了 lazy_encrypt， 则不直接利用Copy，因为有两个阶段：判断阶段和直连阶段
	// 在判断阶段，因为还没确定是否是 tls，所以是要继续用tls加密的，
	// 而直连阶段，只要能让 Copy使用 net.TCPConn的 ReadFrom, 就不用管了， golang最终就会使用splice
	// 之所以可以对拷直连，是因为无论是 socks5 还是vless，只是在最开始的部分 加了目标头，后面的所有tcp连接都是直接传输的数据，就是说，一开始握手什么的是不能直接对拷的，等到后期就可以了
	// 而且之所以能对拷，还有个原因就是，远程服务器 与 客户端 总是源源不断地 为 我们的 原始 TCP 连接 提供数据，我们只是一个中间商而已，左手倒右手

	// 如果开启了  half lazy 开关，则会在 Write的那一端 加强过滤，过滤一些alert，然后 只在Read端 进行splice
	//
	// 如果是客户端，则 从 wlc 读取，写入 wrc ，这种情况是 Write, 然后对于 DetectConn 来说是 Read，即 从DetectConn读取，然后 写入到远程连接
	// 如果是服务端，则 从 wrc 读取，写入 wlc， 这种情况是 Write
	//
	// 总之判断 Write 的对象，是考虑 客户端和服务端之间的数据传输，不考虑 远程真实服务器

	wlcdc := wlc.(*tlsLayer.DetectConn)
	wlccc_raw := wlcdc.RawConn

	var rawWRC *net.TCPConn

	//wrc 有两种情况，如果客户端那就是tls，服务端那就是direct。我们不讨论服务端 处于中间层的情况

	if isclient {
		// 不过实际上客户端 wrc 是 vless的 UserConn， 而UserConn的底层连接才是TLS
		// 很明显，目前我们只支持vless所以才可这么操作，以后再说。

		wrcVless := wrc.(*vless.UserConn)
		tlsConn := wrcVless.Conn.(*tlsLayer.Conn)
		rawWRC = tlsConn.GetRaw()

	} else {
		rawWRC = wrc.(*net.TCPConn) //因为是direct
	}

	if rawWRC == nil {
		log.Println("splice fail reason 0 ")

		//退化回原始状态
		go io.Copy(wrc, wlc)
		io.Copy(wlc, wrc)
		return
	}

	go func() {
		//从 wlccc 读取，向 wrc 写入
		// 此时如果ReadFrom，那就是 wrc.ReadFrom(wlccc)
		//wrc 要实现 ReaderFrom才行, 或者把最底层TCPConn暴露，然后 wlccc 也要把最底层 TCPConn暴露出来
		// 这里就直接采取底层方式

		p := common.GetPacket()
		isgood := false
		isbad := false

		if tls_half_lazy && isclient { // half_lazy时，写入时不使用splice
			isbad = true
		}
		for {
			if isgood || isbad {
				break
			}
			n, err := wlcdc.Read(p)
			if err != nil {
				break
			}
			wrc.Write(p[:n])
			if wlcdc.R.IsTls && wlcdc.RawConn != nil {
				isgood = true
				//log.Println("成功判断 SpliceRead R方向，但是暂时不 直连对拷")
			} else if wlcdc.R.DefinitelyNotTLS {
				isbad = true
			}
		}
		common.PutPacket(p)

		if isbad {
			//直接退化成普通Copy

			if tlsLayer.PDD {
				log.Println("SpliceRead R方向 退化……", wlcdc.R.GetFailReason())
			}
			io.Copy(wrc, wlc)
			return
		}

		if isgood {

			if tlsLayer.PDD {
				log.Println("成功SpliceRead R方向")
				num, e1 := rawWRC.ReadFrom(wlccc_raw)
				log.Println("SpliceRead R方向 读完，", e1, ", 长度:", num)
			} else {
				rawWRC.ReadFrom(wlccc_raw)
			}

		}
	}()

	isgood2 := false
	isbad2 := false
	if tls_half_lazy && !isclient { // half_lazy时，写入时不使用splice
		isbad2 = true
	}
	p := common.GetPacket()

	//从 wrc  读取，向 wlccc 写入
	for {
		if isgood2 || isbad2 {
			break
		}
		n, err := wrc.Read(p)
		if err != nil {
			break
		}
		wlcdc.Write(p[:n])

		if wlcdc.W.IsTls && wlcdc.RawConn != nil {

			if wlcdc.W.LeftTlsRecord != nil {
				//有被分割开的tls包，需要再调用一次write

				// 如果是fit，则 LeftTlsRecord 已经存在了，不能再次阅读

				if wlcdc.W.CutType != tlsLayer.CutType_fit {
					n, err := wrc.Read(p)
					if err != nil {
						break
					}

					// 这个Write会进行内部操作
					wlcdc.Write(p[:n])
					//此时如果没问题的话，Write会把读到部分 和上一次分割开的tlsRecord的那一半合并到一起
					// 而且没再次 使用tls 发送，而是保存到了LeftTlsRecord中 ，供我们 进行直连发送，此时 LeftTlsRecord 的头部是完整的

					// 不过还是不够啊！关键是如何让客户端的 wrc 的tls部分只按record长度读取，不要把我们后面 通过
					//   直连发送的剩余部分给舍弃
					//  因为我们对tls的内部操作缺乏控制; 也许我们可以通过 服务端的Write部分Sleep 1秒来解决，sleep就差不多能确保客户端的tls不会 提前把 我们裸奔的数据也读进缓存
				}

				if tlsLayer.PDD {
					log.Println("SpliceRead W方向, 先通过直连写入之前被分割的tls record", len(wlcdc.W.LeftTlsRecord))
				}

				if wlcdc.W.CutType != tlsLayer.CutType_small { //如果是bigCut/fitCut，则直接通过tcp发送，因为 第一个包之前发过了
					if tls_lazy_fix {
						time.Sleep(time.Millisecond * time.Duration(tls_lazy_milisecond))
					}

					wlccc_raw.Write(wlcdc.W.LeftTlsRecord)
				} else {
					wlcdc.W.SimpleWrite(wlcdc.W.LeftTlsRecord) //如果是SmallCut，则需要通过tls发送，因为包之前一个也没发过

					if tls_lazy_fix {
						time.Sleep(time.Millisecond * time.Duration(tls_lazy_milisecond))
					}
				}

				isgood2 = true

			} else {
				isgood2 = true
			}

		} else if wlcdc.W.DefinitelyNotTLS {
			isbad2 = true
		}
	}
	common.PutPacket(p)

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

		if tlsLayer.PDD {

			num, e2 := wlccc_raw.ReadFrom(rawWRC) //看起来是ReadFrom，实际上是向 wlccc_raw进行Write，即箭头向左
			log.Println("SpliceRead W方向 读完，", e2, ", 长度:", num)
		} else {
			wlccc_raw.ReadFrom(rawWRC)
		}

	}

	//下面处理关闭的情况，因为对拷直连的特殊性，我们代理服务器到 目标服务器的连接关闭后，也即是 rawWRC 被关闭后，可能我们
	// 实际需要
}
