package proxy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/hahahrfool/v2ray_simple/advLayer/grpc"
	"github.com/hahahrfool/v2ray_simple/advLayer/quic"
	"github.com/hahahrfool/v2ray_simple/advLayer/ws"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func PrintAllServerNames() {
	fmt.Printf("===============================\nSupported Proxy Listen protocols:\n")
	for _, v := range utils.GetMapSortedKeySlice(serverCreatorMap) {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

func PrintAllClientNames() {
	fmt.Printf("===============================\nSupported Proxy Dial protocols:\n")

	for _, v := range utils.GetMapSortedKeySlice(clientCreatorMap) {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

// Client 用于向 服务端 拨号.
//服务端是一种 “泛目标”代理，所以我们客户端的 Handshake 要传入目标地址, 来告诉它 我们 想要到达的 目标地址.
// 一个Client 掌握从最底层的tcp等到最上层的 代理协议间的所有数据;
// 一旦一个 Client 被完整定义，则它的数据的流向就被完整确定了.
//
// 然而, udp的转发则不一样. 一般来说, udp只handshake一次, 建立一个通道, 然后在这个通道上
// 不断申请发送到 各个远程udp地址的连接。客户端也可以选择建立多个udp通道。
type Client interface {
	ProxyCommon

	Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error)

	//建立一个通道, 然后在这个通道上 不断申请发送到 各个远程udp地址的连接。
	EstablishUDPChannel(underlay net.Conn, target netLayer.Addr) (netLayer.MsgConn, error)

	//udp的拨号是否使用了多信道方式
	IsUDP_MultiChannel() bool
}

// Server 用于监听 客户端 的连接.
// 服务端是一种 “泛目标”代理，所以我们Handshake要返回 客户端请求的目标地址
// 一个 Server 掌握从最底层的tcp等到最上层的 代理协议间的所有数据;
// 一旦一个 Server 被完整定义，则它的数据的流向就被完整确定了.
type Server interface {
	ProxyCommon

	//ReadWriteCloser 为请求地址为tcp的情况, net.PacketConn 为 请求 建立的udp通道
	Handshake(underlay net.Conn) (io.ReadWriteCloser, netLayer.MsgConn, netLayer.Addr, error)
}

// FullName 可以完整表示 一个 代理的 VSI 层级.
// 这里认为, tcp/udp/kcp/raw_socket 是FirstName，具体的协议名称是 LastName, 中间层是 MiddleName。
//
// An Example of a full name:  tcp+tls+ws+vless
func GetFullName(pc ProxyCommon) string {
	if n := pc.Name(); n == "direct" {
		return n
	} else {
		return pc.Network() + pc.MiddleName() + n

	}
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

	initWS_client() error //for outClient
	initWS_server() error //for inServer

	GetGRPC_Server() *grpc.Server

	initGRPC_server() error

	IsMux() bool //如果用了grpc或者quic, 则此方法返回true

	GetQuic_Client() *quic.Client //for outClient
	setQuic_Client(*quic.Client)

	setListenCommonConnFunc(func() (newConnChan chan net.Conn, baseConn any))

	/////////////////// 内层mux层 ///////////////////

	//0 为不会有 innermux, 1 为有可能有 innermux, 2 为总是使用 innerMux;
	// string 为 innermux内部的 代理 协议 名称
	HasInnerMux() (int, string)

	/////////////////// 其它私有方法 ///////////////////

	setCantRoute(bool)
	setTag(string)
	setAdvancedLayer(string)
	setNetwork(string)

	setIsDial(bool)
	setListenConf(*ListenConf) //for inServer
	setDialConf(*DialConf)     //for outClient

	setPath(string)
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

	quic_c *quic.Client

	listenCommonConnFunc func() (newConnChan chan net.Conn, baseConn any)
}

func (pcs *ProxyCommonStruct) setListenCommonConnFunc(f func() (newConnChan chan net.Conn, baseConn any)) {
	pcs.listenCommonConnFunc = f
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
	var sb strings.Builder
	sb.WriteString("")

	if pcs.TLS {
		sb.WriteString("+tls")
	}
	if pcs.AdvancedL != "" {
		sb.WriteString("+")
		sb.WriteString(pcs.AdvancedL)
	}
	sb.WriteString("+")
	return sb.String()
}

func (pcs *ProxyCommonStruct) CantRoute() bool {
	return pcs.cantRoute
}

//placeholder
func (pcs *ProxyCommonStruct) HasInnerMux() (int, string) {
	return 0, ""
}

//return false
func (pcs *ProxyCommonStruct) IsUDP_MultiChannel() bool {
	return false
}

func (pcs *ProxyCommonStruct) GetTag() string {
	return pcs.Tag
}

func (pcs *ProxyCommonStruct) setTag(tag string) {
	pcs.Tag = tag
}
func (pcs *ProxyCommonStruct) setNetwork(network string) {
	if network == "" {
		pcs.network = "tcp"

	} else {
		pcs.network = network

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

//return nil. As a placeholder.
func (s *ProxyCommonStruct) HandleInitialLayersFunc() func() (newConnChan chan net.Conn, baseConn any) {
	return s.listenCommonConnFunc
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

func (s *ProxyCommonStruct) GetQuic_Client() *quic.Client {
	return s.quic_c
}

func (s *ProxyCommonStruct) setQuic_Client(c *quic.Client) {
	s.quic_c = c
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
func (s *ProxyCommonStruct) initWS_client() error {
	if s.dialConf == nil {
		return errors.New("initWS_client failed when no dialConf assigned")
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

		return utils.ErrInErr{ErrDesc: "initWS_client failed", ErrDetail: e}

	}
	c.UseEarlyData = useEarlyData
	s.ws_c = c

	return nil
}

func (s *ProxyCommonStruct) initWS_server() error {
	if s.listenConf == nil {

		return errors.New("initWS_server failed when no listenConf assigned")

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

	return nil
}

func (s *ProxyCommonStruct) initGRPC_server() error {
	if s.listenConf == nil {

		return errors.New("initGRPC_server failed when no listenConf assigned")

	}

	serviceName := s.listenConf.Path
	if serviceName == "" { //不能为空

		return errors.New("initGRPC_server failed, path must be specified")

	}

	s.grpc_s = grpc.NewServer(serviceName)
	return nil
}
