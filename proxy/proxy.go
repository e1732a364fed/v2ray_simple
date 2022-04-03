package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/grpc"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/quic"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/hahahrfool/v2ray_simple/ws"
	"go.uber.org/zap"
)

func PrintAllServerNames() {
	fmt.Printf("===============================\nSupported Server protocols:\n")
	for v := range serverCreatorMap {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

func PrintAllClientNames() {
	fmt.Printf("===============================\nSupported client protocols:\n")

	for v := range clientCreatorMap {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

// Client 用于向 服务端 拨号.
//服务端是一种 “泛目标”代理，所以我们客户端的 Handshake 要传入目标地址, 来告诉它 我们 想要到达的 目标地址.
// 一个Client 掌握从最底层的tcp等到最上层的 代理协议间的所有数据;
// 一旦一个 Client 被完整定义，则它的数据的流向就被完整确定了.
type Client interface {
	ProxyCommon

	// Handshake的 underlay有可能传入nil，所以要求 所有的 Client 都要能够自己dial
	// 不过目前暂时全在main函数里dial
	Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error)
}

// Server 用于监听 客户端 的连接.
// 服务端是一种 “泛目标”代理，所以我们Handshake要返回 客户端请求的目标地址
// 一个 Server 掌握从最底层的tcp等到最上层的 代理协议间的所有数据;
// 一旦一个 Server 被完整定义，则它的数据的流向就被完整确定了.
type Server interface {
	ProxyCommon

	Handshake(underlay net.Conn) (io.ReadWriteCloser, netLayer.Addr, error)
}

// FullName 可以完整表示 一个 代理的 VSI 层级.
// 这里认为, tcp/udp/kcp/raw_socket 是FirstName，具体的协议名称是 LastName, 中间层是 MiddleName
// An Example of a full name:  tcp+tls+ws+vless
func GetFullName(pc ProxyCommon) string {
	return pc.Network() + pc.MiddleName() + pc.Name()
}

// return GetFullName(pc) + "://" + pc.AddrStr()
func GetVSI_url(pc ProxyCommon) string {
	return GetFullName(pc) + "://" + pc.AddrStr()
}

// 给一个节点 提供 VSI中 第 5-7层 的支持, server和 client通用. 个别方法只能用于某一端.
//
// 一个 ProxyCommon 会内嵌proxy以及上面各层的所有信息;
type ProxyCommon interface {
	Name() string       //代理协议名称, 如vless
	MiddleName() string //其它VSI层 所使用的协议，前后被加了加号，如 +tls+ws+

	Stop()

	/////////////////// 网络层/传输层 ///////////////////

	// 地址,若tcp/udp的话则为 ip:port/host:port的形式, 若是 unix domain socket 则是文件路径 ，
	// 在 inServer就是监听地址，在 outClient就是拨号地址
	AddrStr() string
	SetAddrStr(string)
	Network() string

	CantRoute() bool //for inServer
	GetTag() string

	IsDial() bool //true则为 Dial 端，false 则为 Listen 端
	GetListenConf() *ListenConf
	GetDialConf() *DialConf

	//如果 IsHandleInitialLayers 方法返回true, 则监听/拨号从传输层一直到高级层的过程直接由inServer/outClient自己处理, 而我们主过程直接处理它 处理完毕的剩下的 代理层。
	//
	// quic就属于这种接管底层协议的“超级协议”, 可称之为 SuperProxy。
	IsHandleInitialLayers() bool

	// 在IsHandleInitialLayers时可用， 用于 inServer
	HandleInitialLayersFunc() func() (newConnChan chan net.Conn, baseConn any)

	// 在IsHandleInitialLayers时可用， 用于 outClient
	DialCommonInitialLayerConnFunc() func(serverAddr *netLayer.Addr) any

	// 在IsHandleInitialLayers时可用， 用于 outClient
	DialSubConnFunc() func(any) (net.Conn, error)

	/////////////////// TLS层 ///////////////////

	SetUseTLS()
	IsUseTLS() bool

	GetTLS_Server() *tlsLayer.Server
	GetTLS_Client() *tlsLayer.Client

	setTLS_Server(*tlsLayer.Server)
	setTLS_Client(*tlsLayer.Client)

	/////////////////// http层 ///////////////////
	//默认回落地址.
	GetFallback() *netLayer.Addr
	setFallback(netLayer.Addr)

	CanFallback() bool //如果能fallback，则handshake失败后，可能会专门返回 FallbackErr,如监测到返回了 FallbackErr, 则main函数会进行 回落处理.

	Path() string

	/////////////////// 高级层 ///////////////////

	AdvancedLayer() string //如果使用了ws或者grpc，这个要返回 ws 或 grpc

	GetWS_Client() *ws.Client //for outClient
	GetWS_Server() *ws.Server //for inServer

	initWS_client() //for outClient
	initWS_server() //for inServer

	GetGRPC_Server() *grpc.Server

	initGRPC_server()

	IsMux() bool //如果用了grpc则此方法返回true

	/////////////////// 私有方法 ///////////////////

	setCantRoute(bool)
	setTag(string)
	setAdvancedLayer(string)
	setNetwork(string)

	setIsDial(bool)
	setListenConf(*ListenConf) //for inServer
	setDialConf(*DialConf)     //for outClient

	setPath(string)

	setFunc(index int, thefunc any)
}

//use dc.Host, dc.Insecure, dc.Utls
// 如果用到了quic，还会直接配置quic的client的所有设置.
func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	alpnList := dc.Alpn

	switch com.AdvancedLayer() {
	case "quic":

		com.setNetwork("udp")
		var useHysteria bool
		var maxbyteCount int

		if dc.Extra != nil {
			if thing := dc.Extra["congestion_control"]; thing != nil {
				if use, ok := thing.(string); ok && use == "hy" {
					useHysteria = true

					if thing := dc.Extra["mbps"]; thing != nil {
						if mbps, ok := thing.(int64); ok && mbps > 1 {
							maxbyteCount = int(mbps) * 1024 * 1024 / 8

							log.Println("Using Hysteria Congestion Control, max upload mbps: ", mbps)
						}
					} else {
						log.Println("Using Hysteria Congestion Control, max upload mbps: 3000mbps")
					}
				}
			}

		}

		if len(alpnList) == 0 {
			alpnList = quic.AlpnList
		}

		com.setFunc(proxyCommonStruct_setfunc_DialCommonInitialLayerConn, func(serverAddr *netLayer.Addr) any {

			na, e := netLayer.NewAddr(com.AddrStr())
			if e != nil {
				log.Fatalln("prepareTLS_forClient,quic,netLayer.NewAddr err: ", e)
			}
			return quic.DialCommonInitialLayer(&na, tls.Config{
				InsecureSkipVerify: dc.Insecure,
				ServerName:         dc.Host,
				NextProtos:         alpnList,
				//实测quic的服务端和客户端必须指定alpn, 否则quic客户端会报错
				// CRYPTO_ERROR (0x178): ALPN negotiation failed. Server didn't offer any protocols
			}, useHysteria, maxbyteCount)
		})

		com.setFunc(proxyCommonStruct_setfunc_DialSubConn, func(t any) (net.Conn, error) {

			return quic.DialSubConn(t)
		})

		return nil //quic直接接管了tls，所以不执行下面步骤

	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}
	com.setTLS_Client(tlsLayer.NewClient(dc.Host, dc.Insecure, dc.Utls, alpnList))
	return nil
}

//use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure
// 如果用到了quic，还会直接配置quic的server的所有设置.
func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {
	// 这里直接不检查 字符串就直接传给 tlsLayer.NewServer
	// 所以要求 cert和 key 不在程序本身目录 的话，就要给出完整路径

	alpnList := lc.Alpn
	switch com.AdvancedLayer() {
	case "quic":

		com.setNetwork("udp")

		if len(alpnList) == 0 {
			alpnList = quic.AlpnList
		}

		var useHysteria bool
		var maxbyteCount int

		if lc.Extra != nil {
			if thing := lc.Extra["congestion_control"]; thing != nil {
				if use, ok := thing.(string); ok && use == "hy" {
					useHysteria = true

					if thing := lc.Extra["mbps"]; thing != nil {
						if mbps, ok := thing.(int64); ok && mbps > 1 {
							maxbyteCount = int(mbps) * 1024 * 1024 / 8

							log.Println("Using Hysteria Congestion Control, max upload mbps: ", mbps)

						}
					} else {

						log.Println("Using Hysteria Congestion Control, max upload mbps: 3000mbps")

					}

				}
			}

		}

		com.setFunc(proxyCommonStruct_setfunc_HandleInitialLayers, func() (newConnChan chan net.Conn, baseConn any) {

			certArray, err := tlsLayer.GetCertArrayFromFile(lc.TLSCert, lc.TLSKey)

			if err != nil {
				log.Fatalf("can't create tls cert from file: %s, %s, %s\n", lc.TLSCert, lc.TLSKey, err)
			}

			return quic.ListenInitialLayers(com.AddrStr(), tls.Config{
				InsecureSkipVerify: lc.Insecure,
				ServerName:         lc.Host,
				Certificates:       certArray,
				NextProtos:         alpnList,
			}, useHysteria, maxbyteCount)

		})

		return nil //quic直接接管了tls，所以不执行下面步骤
	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}

	tlsserver, err := tlsLayer.NewServer(lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, alpnList)
	if err == nil {
		com.setTLS_Server(tlsserver)
	} else {
		return err
	}
	return nil
}

//给 ProxyCommon 的tls做一些配置上的准备，从url读取配置
func prepareTLS_forProxyCommon_withURL(u *url.URL, isclient bool, com ProxyCommon) error {
	insecureStr := u.Query().Get("insecure")
	insecure := false
	if insecureStr != "" && insecureStr != "false" && insecureStr != "0" {
		insecure = true
	}

	if isclient {
		utlsStr := u.Query().Get("utls")
		useUtls := utlsStr != "" && utlsStr != "false" && utlsStr != "0"
		com.setTLS_Client(tlsLayer.NewClient(u.Host, insecure, useUtls, nil))

	} else {
		certFile := u.Query().Get("cert")
		keyFile := u.Query().Get("key")

		hostAndPort := u.Host
		sni, _, _ := net.SplitHostPort(hostAndPort)

		tlsserver, err := tlsLayer.NewServer(sni, certFile, keyFile, insecure, nil)
		if err == nil {
			com.setTLS_Server(tlsserver)
		} else {
			return err
		}
	}
	return nil
}

// ProxyCommonStruct 实现 ProxyCommon中除了Name 之外的其他方法.
// 规定，所有的proxy都要内嵌本struct
// 这是verysimple的架构所要求的。
// verysimple规定，在加载完配置文件后，一个listen和一个dial所使用的全部层级都是确定了的.
//  因为所有使用的层级都是确定的，就可以进行针对性优化
type ProxyCommonStruct struct {
	Addr    string
	TLS     bool
	Tag     string //可用于路由, 见 netLayer.route.go
	network string

	PATH string

	tls_s *tlsLayer.Server
	tls_c *tlsLayer.Client

	isdial     bool
	listenConf *ListenConf
	dialConf   *DialConf

	cantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	ws_c *ws.Client
	ws_s *ws.Server

	grpc_s       *grpc.Server
	FallbackAddr *netLayer.Addr

	listenCommonConnFunc func() (newConnChan chan net.Conn, baseConn any)
	dialCommonConnFunc   func(serverAddr *netLayer.Addr) any
	dialSubConnFunc      func(any) (net.Conn, error)
}

func (pcs *ProxyCommonStruct) Network() string {
	return pcs.network
}

func (pcs *ProxyCommonStruct) Path() string {
	return pcs.PATH
}
func (pcs *ProxyCommonStruct) setPath(a string) {
	pcs.PATH = a
}

func (pcs *ProxyCommonStruct) GetFallback() *netLayer.Addr {
	return pcs.FallbackAddr
}
func (pcs *ProxyCommonStruct) setFallback(a netLayer.Addr) {
	pcs.FallbackAddr = &a
}

func (pcs *ProxyCommonStruct) MiddleName() string {
	str := ""
	if pcs.TLS {
		str += "+tls"
	}
	if pcs.AdvancedL != "" {
		str += "+" + pcs.AdvancedL
	}
	return str + "+"
}

func (pcs *ProxyCommonStruct) CantRoute() bool {
	return pcs.cantRoute
}

func (pcs *ProxyCommonStruct) GetTag() string {
	return pcs.Tag
}

func (pcs *ProxyCommonStruct) setTag(tag string) {
	pcs.Tag = tag
}
func (pcs *ProxyCommonStruct) setNetwork(net string) {
	if net == "" {
		pcs.network = "tcp"

	} else {
		pcs.network = net

	}
}

func (pcs *ProxyCommonStruct) setCantRoute(cr bool) {
	pcs.cantRoute = cr
}

func (pcs *ProxyCommonStruct) setAdvancedLayer(adv string) {
	pcs.AdvancedL = adv
}

func (pcs *ProxyCommonStruct) AdvancedLayer() string {
	return pcs.AdvancedL
}

//do nothing. As a placeholder.
func (s *ProxyCommonStruct) Stop() {
}

//return false. As a placeholder.
func (s *ProxyCommonStruct) CanFallback() bool {
	return false
}

//return false. As a placeholder.
func (s *ProxyCommonStruct) IsHandleInitialLayers() bool {
	return s.AdvancedL == "quic"
}

//return nil. As a placeholder.
func (s *ProxyCommonStruct) HandleInitialLayersFunc() func() (newConnChan chan net.Conn, baseConn any) {
	return s.listenCommonConnFunc
}

//return nil. As a placeholder.
func (s *ProxyCommonStruct) DialCommonInitialLayerConnFunc() func(serverAddr *netLayer.Addr) any {
	return s.dialCommonConnFunc
}

//return nil. As a placeholder.
func (s *ProxyCommonStruct) DialSubConnFunc() func(any) (net.Conn, error) {
	return s.dialSubConnFunc
}

const (
	proxyCommonStruct_setfunc_HandleInitialLayers = iota
	proxyCommonStruct_setfunc_DialCommonInitialLayerConn
	proxyCommonStruct_setfunc_DialSubConn
)

func (s *ProxyCommonStruct) setFunc(index int, thefunc any) {
	switch index {
	case proxyCommonStruct_setfunc_HandleInitialLayers:
		s.listenCommonConnFunc = thefunc.(func() (newConnChan chan net.Conn, baseConn any))
	case proxyCommonStruct_setfunc_DialCommonInitialLayerConn:
		s.dialCommonConnFunc = thefunc.(func(serverAddr *netLayer.Addr) any)
	case proxyCommonStruct_setfunc_DialSubConn:
		s.dialSubConnFunc = thefunc.(func(any) (net.Conn, error))
	}
}

func (pcs *ProxyCommonStruct) setTLS_Server(s *tlsLayer.Server) {
	pcs.tls_s = s
}
func (s *ProxyCommonStruct) setTLS_Client(c *tlsLayer.Client) {
	s.tls_c = c
}

func (s *ProxyCommonStruct) GetTLS_Server() *tlsLayer.Server {
	return s.tls_s
}
func (s *ProxyCommonStruct) GetTLS_Client() *tlsLayer.Client {
	return s.tls_c
}

func (s *ProxyCommonStruct) AddrStr() string {
	return s.Addr
}
func (s *ProxyCommonStruct) SetAddrStr(a string) {
	s.Addr = a
}

func (s *ProxyCommonStruct) IsUseTLS() bool {
	return s.TLS
}

func (s *ProxyCommonStruct) IsMux() bool {
	switch s.AdvancedL {
	case "grpc", "quic":
		return true
	}
	return false
}

func (s *ProxyCommonStruct) SetUseTLS() {
	s.TLS = true
}
func (s *ProxyCommonStruct) setIsDial(b bool) {
	s.isdial = b
}
func (s *ProxyCommonStruct) setListenConf(lc *ListenConf) {
	s.listenConf = lc
}
func (s *ProxyCommonStruct) setDialConf(dc *DialConf) {
	s.dialConf = dc
}

//true则为 Dial 端，false 则为 Listen 端
func (s *ProxyCommonStruct) IsDial() bool {
	return s.isdial
}
func (s *ProxyCommonStruct) GetListenConf() *ListenConf {
	return s.listenConf
}
func (s *ProxyCommonStruct) GetDialConf() *DialConf {
	return s.dialConf
}

//for outClient
func (s *ProxyCommonStruct) GetWS_Client() *ws.Client {
	return s.ws_c
}
func (s *ProxyCommonStruct) GetWS_Server() *ws.Server {
	return s.ws_s
}

func (s *ProxyCommonStruct) GetGRPC_Server() *grpc.Server {
	return s.grpc_s
}

//for outClient
func (s *ProxyCommonStruct) initWS_client() {
	if s.dialConf == nil {
		const eStr = "initWS_client failed when no dialConf assigned"
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal(eStr)
		} else {
			log.Fatal(eStr)

		}
	}
	path := s.dialConf.Path
	if path == "" { // 至少Path需要为 "/"
		path = "/"
	}

	var useEarlyData bool
	if s.dialConf.Extra != nil {
		if thing := s.dialConf.Extra["ws_earlydata"]; thing != nil {
			if use, ok := thing.(bool); ok && use {
				useEarlyData = true
			}
		}
	}

	c, e := ws.NewClient(s.dialConf.GetAddrStr(), path)
	if e != nil {

		const eStr2 = "initWS_client failed"
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal(eStr2, zap.Error(e))
		} else {
			log.Fatal(eStr2, e)

		}

	}
	c.UseEarlyData = useEarlyData
	s.ws_c = c

}

func (s *ProxyCommonStruct) initWS_server() {
	if s.listenConf == nil {

		const eStr3 = "initWS_server failed when no listenConf assigned"
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal(eStr3)
		} else {
			log.Fatal(eStr3)
		}

	}
	path := s.listenConf.Path
	if path == "" { // 至少Path需要为 "/"
		path = "/"
	}

	var useEarlyData bool
	if s.listenConf.Extra != nil {
		if thing := s.listenConf.Extra["ws_earlydata"]; thing != nil {
			if use, ok := thing.(bool); ok && use {
				useEarlyData = true
			}
		}
	}
	wss := ws.NewServer(path)
	wss.UseEarlyData = useEarlyData

	s.ws_s = wss
}

func (s *ProxyCommonStruct) initGRPC_server() {
	if s.listenConf == nil {

		const eStr1 = "initGRPC_server failed when no listenConf assigned"
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal(eStr1)
		} else {
			log.Fatal(eStr1)
		}

	}

	serviceName := s.listenConf.Path
	if serviceName == "" { //不能为空

		const eStr2 = "initGRPC_server failed, path must be specified"
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal(eStr2)
		} else {
			log.Fatal(eStr2)
		}
	}

	s.grpc_s = grpc.NewServer(serviceName)
}
