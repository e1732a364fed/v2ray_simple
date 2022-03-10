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

	"github.com/hahahrfool/v2ray_simple/proxy"
)

var (
	version = "1.2.0"
	desc    = "v2ray_simple, a simple implementation of V2Ray, 并且在某些地方试图走在v2ray前面"

	configFileName = flag.String("c", "client.json", "config file name")

	conf         *Config
	directClient proxy.Client
)

func init() {
	directClient, _ = proxy.ClientFromURL("direct://")

}

func printVersion() {
	fmt.Printf("===============================\nv2ray_simple %v (%v), %v %v %v\n", version, desc, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	proxy.PrintAllServerNames()

	proxy.PrintAllClientNames()

	fmt.Printf("===============================\n")
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

	var err error

	// 读取配置文件，默认为客户端模式
	conf, err = loadConfig(*configFileName)
	if err != nil {
		log.Println("can not load config file: ", err)
		os.Exit(-1)
	}

	// 根据配置文件初始化组件
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

	// 后台运行
	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals
	}
}

func handleNewIncomeConnection(localServer proxy.Server, remoteClient proxy.Client, thisLocalConnectionInstance net.Conn) {

	log.Println("got new")

	var err error
	if localServer.IsUseTLS() {
		tlsConn, err := localServer.GetTLS_Server().Handshake(thisLocalConnectionInstance)
		if err != nil {
			log.Println("failed in handshake localServer tls", localServer.AddrStr(), err)
			thisLocalConnectionInstance.Close()
			return
		}

		thisLocalConnectionInstance = tlsConn
	}

	// 不同的服务端协议各自实现自己的响应逻辑, 其中返回的地址则用于匹配路由
	// 常常需要额外编解码或者流量统计的功能，故需要给lc包一层以实现这些逻辑，即wlc
	wlc, targetAddr, err := localServer.Handshake(thisLocalConnectionInstance)
	if err != nil {
		log.Printf("failed in handshake from %v: %v", localServer.AddrStr(), err)
		thisLocalConnectionInstance.Close()
		return
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
		log.Println("will dial", client.AddrStr())

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

	// 不同的客户端协议各自实现自己的请求逻辑
	wrc, err := client.Handshake(clientConn, targetAddr)
	if err != nil {
		log.Println("failed in handshake to", targetAddr.String(), ", Reason: ", err)
		return
	}

	// 流量转发
	go func() {
		n, e := io.Copy(wrc, wlc)
		log.Println("本地->远程 转发结束", realTargetAddr.String(), n, e)
	}()
	n, e := io.Copy(wlc, wrc)

	log.Println("远程->本地 转发结束", realTargetAddr.String(), n, e)

}
