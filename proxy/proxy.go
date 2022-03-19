package proxy

import (
	"fmt"
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
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

// Client 用于向 服务端 拨号
type Client interface {
	ProxyCommon

	Name() string

	// Handshake的 underlay有可能传入nil，所以要求 所有的 Client 都要能够自己dial
	Handshake(underlay net.Conn, target *netLayer.Addr) (io.ReadWriter, error)
}

// Server 用于监听 客户端 的连接
type Server interface {
	ProxyCommon

	Name() string

	Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error)
	Stop()

	CanFallback() bool //如果能fallback，则handshake失败后，可能会专门返回 FallbackErr,如监测到返回了 FallbackErr, 则main函数会进行 回落处理.
}

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

func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	com.setTLS_Client(tlsLayer.NewTlsClient(dc.Host, dc.Insecure, dc.Utls))
	return nil
}

func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {
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

// ProxyCommonStruct 实现 ProxyCommon
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
