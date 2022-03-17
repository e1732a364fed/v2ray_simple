/*
Package proxy 定义了代理转发所需的必备组件

Layer Definition 层级讨论

目前认为，一个传输过程由四个部分组成，基础连接（udp/tcp），TLS（可选），中间层（ws、grpc、http等，可选），具体协议（socks5，vless，trojan等）.

其中，ws和grpc被认为是 高级应用层，http（伪装）属于低级应用层.

TLS - Transport Layer Security 顾名思义TLS作用于传输层，第四层，但是我们tcp也是第四层，所以在本项目中，认为不需要“会话层”，单独加一个专用于tls的层比较稳妥.

正常OSI是7层，我们在这里规定一个 第八层和第九层，第八层就是 vless协议所在位置，第九层就是我们实际传输的承载数据.


New Model - VSI 新的VSI 模型

那么我们提出一个 verysimple Interconnection Model， 简称vsi模型。1到4层与OSI相同（物理、链路、网络、传输).

把第五层替换成“加密层”，把TLS放进去；把第六层改为低级应用层，http属于这一层.

第七层 改为高级应用层，ws/grpc 属于这一层, 简称高级层；第八层定为 代理层，vless/trojan 在这层.

第九层为 承载数据层，承载的为 另一大串 第四层的数据.

那么我们verysimple实际上就是 基于 “层” 的架构，或称 可分层结构.


	9 ｜ tcp data
	--------------------
	8 ｜ vless/trojan/socks5
	--------------------
	7 ｜ ws/grpc
	--------------------
	6 ｜ http
	--------------------
	5 ｜ tls
	--------------------
	4 ｜ tcp
	--------------------


基本上5-8层都是可控的.

对应的理想配置文件应该如下.

	{
		"layer5_settings": {	//或者叫 tls_settings，
			"tls":{"insecure": true},
			"utls":{}
		},
		"layer6_settings": {	//或者叫 http_settings
			"headers":{}
		},
		"layer7_settings": {	//或者叫 advancedLayer_settings
			"ws":{}
		},
		"layer8_settings": {	//或者叫 proxy_settings
			"vless":{}
			"trojan":{}
		},
	}

我们项目的文件夹，proxy文件夹代表第8层，tlsLayer文件夹代表第5层; 未来还计划在根目录下添加http文件夹（第六层），
ws和grpc文件夹（第七层）.


同级的ws和grpc是独占的，可以都放到一个layer里，然后比如第八层配置了一个vless一个trojan，那么排列组合就是4种，vless+ws, vless+ grpc, trojan+ws, trojan+grpc.


这就大大减轻了各种”一键脚本“的 使用需求，咱们只要选择自己喜欢的各个层，程序自动就为我们生成所有配置.

运行时，如果所有配置都要有，那么就需要多种端口；共用端口的话可以用nginx.

也可以程序指定一种 特定的情况，比如开始运行程序时，冒出交互界面，自己按项选择好后，就自动运行，然后自动生成客户端分享url.

可以在脑海里想象 “穿鞋带” 的画面，有很多洞可以经过，都穿好了，鞋带就系好了。或者手机手势解锁的情况.

这种好处是，每次运行都可以采用不同的配置，不同的uuid，手机一扫码就能连上.

Contents of proxy package - proxy包内容

接口 ProxyCommon 和 结构 ProxyCommonStruct 给 这个架构定义了标准.

而 Client 和 Server 接口 是 具体利用该架构的 客户端 和 服务端，都位于VSI中的第八层.

使用 RegisterClient 和 RegisterServer 来注册新的实现.

还定义了关于udp 转发到机制，该部分直接参考 各个UDP开头的 部分即可.

Server and Client

我们服务端和 客户端的程序，都是有至少一个入口和一个出口的。入口我们叫做 inServer ，出口我们叫做 outClient.

这两个词的含义和 v2ray的 inbound 和 outbound 是等价的.

在 inServer 中，我们负责监听未知连接；在 outClient 中，我们负责拨号特定目标服务器.

*/
package proxy

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

// 给一个节点 提供 VSI中 第 5-7层 的支持, server和 client通用. 个别方法只能用于某一端
// 一个 ProxyCommon 会内嵌proxy以及上面各层的所有信息
type ProxyCommon interface {
	AddrStr() string //地址，在server就是监听地址，在client就是拨号地址
	SetAddrStr(string)
	CantRoute() bool //for inServer
	GetTag() string

	SetUseTLS()
	IsUseTLS() bool

	HasAdvancedApplicationLayer() bool //如果使用了ws或者grpc，这个要返回true

	GetTLS_Server() *tlsLayer.Server
	GetTLS_Client() *tlsLayer.Client

	setTLS_Server(*tlsLayer.Server)
	setTLS_Client(*tlsLayer.Client)

	setCantRoute(bool)
	setTag(string)
}

//给 ProxyCommon 的tls做一些配置上的准备，从url读取配置
func prepareTLS_forProxyCommon(u *url.URL, isclient bool, com ProxyCommon) error {
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

type ProxyCommonStruct struct {
	Addr string
	TLS  bool
	Tag  string //可用于路由, 见 netLayer.route.go

	tls_s *tlsLayer.Server
	tls_c *tlsLayer.Client

	cantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出
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

func (pcs *ProxyCommonStruct) setCantRoute(cr bool) {
	pcs.cantRoute = cr
}

func (pcs *ProxyCommonStruct) HasAdvancedApplicationLayer() bool {
	return false
}

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

// Client 用于向 服务端 拨号
type Client interface {
	ProxyCommon

	Name() string

	// Handshake的 underlay有可能传入nil，所以要求 所有的 Client 都要能够自己dial
	Handshake(underlay net.Conn, target *netLayer.Addr) (io.ReadWriter, error)
}

// ClientCreator is a function to create client.
type ClientCreator func(url *url.URL) (Client, error)

var (
	clientMap = make(map[string]ClientCreator)
)

// RegisterClient is used to register a client.
func RegisterClient(name string, c ClientCreator) {
	clientMap[name] = c
}

// ClientFromURL calls the registered creator to create client.
// dialer is the default upstream dialer so cannot be nil, we can use Default when calling this function.
func ClientFromURL(s string) (Client, error) {
	u, err := url.Parse(s)
	if err != nil {

		return nil, utils.NewDataErr("can not parse client url", err, s)
	}

	schemeName := strings.ToLower(u.Scheme)

	creatorFunc, ok := clientMap[schemeName]
	if ok {
		return creatorFunc(u)
	} else {

		//尝试判断是否套tls, 比如vlesss实际上是vless+tls，https实际上是http+tls

		realScheme := strings.TrimSuffix(schemeName, "s")
		creatorFunc, ok = clientMap[realScheme]
		if ok {
			c, err := creatorFunc(u)
			if err != nil {
				return c, err
			}

			c.SetUseTLS()
			prepareTLS_forProxyCommon(u, true, c)

			return c, err

		}

	}

	return nil, utils.NewDataErr("unknown client scheme '", nil, u.Scheme)
}

// Server 用于监听 客户端 的连接
type Server interface {
	ProxyCommon

	Name() string

	Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error)
	Stop()

	CanFallback() bool //如果能fallback，则handshake失败后，会专门返回 ErrFallback
}

// ServerCreator is a function to create proxy server
type ServerCreator func(url *url.URL) (Server, error)

var (
	serverMap = make(map[string]ServerCreator)
)

// RegisterServer is used to register a proxy server
func RegisterServer(name string, c ServerCreator) {
	serverMap[name] = c
}

func PrintAllServerNames() {
	fmt.Println("===============================\nSupported Server protocols:")
	for v := range serverMap {
		fmt.Println(v)
	}
}

func PrintAllClientNames() {
	fmt.Println("===============================\nSupported client protocols:")

	for v := range clientMap {
		fmt.Println(v)
	}
}

// ServerFromURL calls the registered creator to create proxy servers
// dialer is the default upstream dialer so cannot be nil, we can use Default when calling this function
// 所有的server都可有 "norule"参数，标明无需路由或者此server不可使用路由，在监听多个ip时是有用的;
// 路由配置可以在json的其他配置里面设置，如 mycountry项
func ServerFromURL(s string) (Server, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, utils.NewDataErr("can not parse server url ", err, s)
	}

	schemeName := strings.ToLower(u.Scheme)
	creatorFunc, ok := serverMap[schemeName]
	if ok {
		ser, err := creatorFunc(u)
		if err != nil {
			return nil, err
		}
		configCommonForServer(ser, u)

		return ser, nil
	} else {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creatorFunc, ok = serverMap[realScheme]
		if ok {
			server, err := creatorFunc(u)
			if err != nil {
				return nil, err
			}
			configCommonForServer(server, u)

			server.SetUseTLS()
			prepareTLS_forProxyCommon(u, false, server)
			return server, nil

		}
	}

	return nil, utils.NewDataErr("unknown server scheme '", nil, u.Scheme)
}

func configCommonForServer(ser ProxyCommon, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("norule") != "" {
		nr = true
	}
	ser.setCantRoute(nr)
	ser.setTag(u.Fragment)

}
