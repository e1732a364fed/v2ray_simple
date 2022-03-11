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

	configFileName = flag.String("c", "client.json", "config file name")

	conf         *Config
	directClient proxy.Client

	tls_lazy_encryptPtr = flag.Bool("lazy", false, "tls lazy encrypt (splice)")
	tls_lazy_encrypt    bool
)

func init() {
	directClient, _ = proxy.ClientFromURL("direct://")

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
				return nil, fmt.Errorf("can not parse config file %v, %v", configFileName, err)
			}
			return config, nil
		}
	}
	return nil, fmt.Errorf("can not load config file %v", configFileName)
}

func main() {

	printVersion()

	flag.Parse()
	tls_lazy_encrypt = *tls_lazy_encryptPtr

	var err error

	conf, err = loadConfig(*configFileName)
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
	// TODO: 未来main函数可以推出 交互模式，未来推出动态增删用户、查询流量等功能时就有用
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

	log.Println("got new", thisLocalConnectionInstance.RemoteAddr().String())

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
		log.Printf("failed in handshake from %v: %v", localServer.AddrStr(), err)
		thisLocalConnectionInstance.Close()
		return
	}

	// 我们在客户端 lazy_encrypt 探测时，读取socks5 传来的信息，因为这个和要发送到tls的信息时一模一样的，所以就不需要等包上vless、tls后再判断了, 直接解包 socks5进行判断
	//
	//  而在服务端探测时，因为包了 tls，所以要在tls解包后, vless 解包后，再进行判断；
	// 所以总之都是要在 localServer判断 wlc，只不过理由不一样

	var thecc *tlsLayer.CopyConn

	if tls_lazy_encrypt {

		thecc = tlsLayer.NewDetectConn(baseLocalConn, wlc)

		wlc = thecc
		//clientConn = cc
	}

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
		defer thisLocalConnectionInstance.Close()

		// 这里 因为 vless v1 的 CRUMFURS 信道 会对 thisLocalConnectionInstance 出现keep alive ，
		// 而且具体的传递信息的部分并不是在main函数中处理，而是自己的go routine，所以不能直接关闭 thisLocalConnectionInstance，
		// 所以要分情况进行 defer thisLocalConnectionInstance.Close()。
	}

	var client proxy.Client

	client = remoteClient

	log.Printf("%s want to dial %s", client.Name(), targetAddr.UrlString())

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
		tryRaw(wrc, wlc, client.IsUseTLS())
		return
	}

	go io.Copy(wrc, wlc)
	io.Copy(wlc, wrc)

}

//和 xtls的splice 含义相同
func tryRaw(wrc, wlc io.ReadWriter, isclient bool) {

	//如果用了 lazy_encrypt， 则不直接利用Copy，因为有两个阶段：判断阶段和直连阶段
	// 在判断阶段，因为还没确定是否是 tls，所以是要继续用tls加密的，
	// 而直连阶段，只要能让 Copy使用 ReadFrom, 就能一步一步最终使用splice了

	//首先判断我们的wlc（*tlsLayer.CopyConn) 是否得出来 IsTLS
	wlccc := wlc.(*tlsLayer.CopyConn)
	wlccc_raw := wlccc.RawConn

	var rawWRC *net.TCPConn

	//wrc 有两种情况，如果客户端那就是tls，服务端那就是direct。我们不讨论服务端 处于中间层的情况

	if isclient {
		// 不过实际上 wrc 是 vless的 UserConn， 而UserConn的底层连接才是TLS

		wrcVless := wrc.(*vless.UserConn)

		tlsConn := wrcVless.Conn.(*tlsLayer.Conn)

		rawWRC = tlsConn.GetRaw()

	} else {
		rawWRC = wrc.(*net.TCPConn)
	}

	if rawWRC == nil {
		log.Println("splice fail reason 3 ")
		io.Copy(wrc, wlc)
		return
	}

	go func() {
		//从 wlccc 读取，向 wrc 写入
		// 此时如果ReadFrom，那就是 wrc.ReadFrom(wlccc)
		//wrc 要实现 ReaderFrom才行, 或者把最底层TCPConn暴露，然后 wlccc 也要把最底层 TCPConn暴露出来

		p := common.GetPacket()
		isgood := false
		for {

			if isgood {
				break
			}

			n, err := wlccc.Read(p)
			if err != nil {

				break
			}
			wrc.Write(p[:n])

			if wlccc.R.IsTls && wlccc.RawConn != nil {
				isgood = true

			}
		}
		if isgood {

			log.Println("成功SpliceRead 方向1")
			rawWRC.ReadFrom(wlccc_raw)

		}
	}()

	isgood2 := false
	p := common.GetPacket()

	//从 wrc  读取，向 wlccc 写入
	for {
		if isgood2 {
			break
		}
		n, err := wrc.Read(p)
		if err != nil {

			break
		}
		wlccc.Write(p[:n])
		if wlccc.W.IsTls && wlccc.RawConn != nil {
			isgood2 = true

		}
	}
	if isgood2 {
		log.Println("成功SpliceRead 方向2")
		wlccc_raw.ReadFrom(rawWRC)
	}
}
