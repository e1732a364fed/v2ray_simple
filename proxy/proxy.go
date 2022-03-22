package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/ws"
)

func PrintAllServerNames() {
	fmt.Println("===============================\nSupported Server protocols:")
	for v := range serverCreatorMap {
		fmt.Println(v)
	}
}

func PrintAllClientNames() {
	fmt.Println("===============================\nSupported client protocols:")

	for v := range clientCreatorMap {
		fmt.Println(v)
	}
}

// Client 用于向 服务端 拨号. 服务端是一种 “泛目标”代理，所以我们Handshake要传入目标地址
// 一个Client 掌握从最底层的tcp等到最上层的 代理协议间的所有数据
type Client interface {
	ProxyCommon

	// Handshake的 underlay有可能传入nil，所以要求 所有的 Client 都要能够自己dial
	// 不过目前暂时全在main函数里dial
	Handshake(underlay net.Conn, target *netLayer.Addr) (io.ReadWriter, error)
}

// Server 用于监听 客户端 的连接.
// 服务端是一种 “泛目标”代理，所以我们Handshake要返回 客户端请求的目标地址
// 一个 Server 掌握从最底层的tcp等到最上层的 代理协议间的所有数据
type Server interface {
	ProxyCommon

	Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error)
	Stop()

	CanFallback() bool //如果能fallback，则handshake失败后，可能会专门返回 FallbackErr,如监测到返回了 FallbackErr, 则main函数会进行 回落处理.
}

// FullName 可以完整表示 一个 代理的 VSI 层级.
// 这里认为, tcp/udp/kcp/raw_socket 是FirstName，具体的协议名称是 LastName, 中间层是 MiddleName
// An Example of a full name:  tcp+tls+ws+vless
func GetFullName(pc ProxyCommon) string {

	return pc.Network() + pc.MiddleName() + pc.Name()

}

// 给一个节点 提供 VSI中 第 5-7层 的支持, server和 client通用. 个别方法只能用于某一端
// 一个 ProxyCommon 会内嵌proxy以及上面各层的所有信息
type ProxyCommon interface {
	Name() string       //代理协议名称, 如vless
	MiddleName() string //其它VSI层 所使用的协议，如 +tls+ws

	/////////////////// 网络层/传输层 ///////////////////

	// 地址,若tcp/udp的话则为 ip:port/host:port的形式, 若是uds则是文件路径 ，
	// 在server就是监听地址，在client就是拨号地址
	AddrStr() string
	SetAddrStr(string)
	Network() string

	CantRoute() bool //for inServer
	GetTag() string

	IsDial() bool //true则为 Dial 端，false 则为 Listen 端
	GetListenConf() *ListenConf
	GetDialConf() *DialConf

	/////////////////// TLS层 ///////////////////

	SetUseTLS()
	IsUseTLS() bool

	GetTLS_Server() *tlsLayer.Server
	GetTLS_Client() *tlsLayer.Client

	setTLS_Server(*tlsLayer.Server)
	setTLS_Client(*tlsLayer.Client)

	/////////////////// 高级层 ///////////////////

	AdvancedLayer() string //如果使用了ws或者grpc，这个要返回 ws 或 grpc

	GetWS_Client() *ws.Client //for outClient
	GetWS_Server() *ws.Server //for inServer

	initWS_client() //for outClient
	initWS_server() //for inServer

	setCantRoute(bool)
	setTag(string)
	setAdvancedLayer(string)
	setNetwork(string)

	setIsDial(bool)
	setListenConf(*ListenConf) //for inServer
	setDialConf(*DialConf)     //for outClient

}

//use dc.Host, dc.Insecure, dc.Utls
func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	com.setTLS_Client(tlsLayer.NewTlsClient(dc.Host, dc.Insecure, dc.Utls))
	return nil
}

//use lc.GetAddr(), lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure
func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {
	// 这里直接不检查 字符串就直接传给 tlsLayer.NewServer
	// 所以要求 cert和 key 不在程序本身目录 的话，就要给出完整路径
	tlsserver, err := tlsLayer.NewServer(lc.GetAddr(), lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure)
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
		com.setTLS_Client(tlsLayer.NewTlsClient(u.Host, insecure, useUtls))

	} else {
		certFile := u.Query().Get("cert")
		keyFile := u.Query().Get("key")

		hostAndPort := u.Host
		sni, _, _ := net.SplitHostPort(hostAndPort)

		tlsserver, err := tlsLayer.NewServer(hostAndPort, sni, certFile, keyFile, insecure)
		if err == nil {
			com.setTLS_Server(tlsserver)
		} else {
			return err
		}
	}
	return nil
}

// ProxyCommonStruct 实现 ProxyCommon中除了Name 之外的其他方法
// 规定，所有的proxy都要内嵌本struct
// 这是verysimple的架构所要求的。verysimple规定，在加载完配置文件后，一个listen和一个dial所使用的全部层级都是确定了的
//  因为所有使用的层级都是确定的，就可以进行针对性优化
type ProxyCommonStruct struct {
	Addr    string
	TLS     bool
	Tag     string //可用于路由, 见 netLayer.route.go
	network string

	tls_s *tlsLayer.Server
	tls_c *tlsLayer.Client

	isdial     bool
	listenConf *ListenConf
	dialConf   *DialConf

	cantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	ws_c *ws.Client
	ws_s *ws.Server
}

func (pcs *ProxyCommonStruct) Network() string {
	return pcs.network
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

// 从 url 初始化一些通用的配置，目前只有 u.Host
func (pcs *ProxyCommonStruct) InitFromUrl(u *url.URL) {
	pcs.Addr = u.Host
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

//for outClient
func (s *ProxyCommonStruct) initWS_client() {
	if s.dialConf == nil {
		log.Fatal("initWS_client failed when no dialConf assigned")
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

	c, e := ws.NewClient(s.dialConf.GetAddr(), path)
	if e != nil {
		log.Fatal("initWS_client failed", e)
	}
	c.UseEarlyData = useEarlyData
	s.ws_c = c

}

func (s *ProxyCommonStruct) initWS_server() {
	if s.listenConf == nil {
		log.Fatal("initWS_server failed when no listenConf assigned")
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
