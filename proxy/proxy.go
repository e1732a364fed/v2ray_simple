package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"strings"
	"time"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

//配置文件格式
const (
	SimpleMode = iota
	StandardMode
	V2rayCompatibleMode
)

//规定，如果 proxy的server的handshake如果返回的是具有内层mux的连接，该连接要实现 MuxMarker 接口.
type MuxMarker interface {
	io.ReadWriteCloser
	IsMux()
}

//实现 MuxMarker
type MuxMarkerConn struct {
	netLayer.ReadWrapper
}

func (mh *MuxMarkerConn) IsMux() {}

//有的客户端可能建立tcp连接后首先由读服务端的数据？虽然比较少见但是确实存在
// 总之 firstpayload是有可能读不到的，我们尽量减少这个延迟.
// 也有可能是有人通过 nc 来测试，也会遇到这种读不到 firstpayload的情况
const FirstPayloadTimeout = time.Millisecond * 100

// Client 用于向 服务端 拨号.
//服务端是一种 “泛目标”代理，所以我们客户端的 Handshake 要传入目标地址, 来告诉它 我们 想要到达的 目标地址.
// 一个Client 掌握从最底层的tcp等到最上层的 代理协议间的所有数据;
// 一旦一个 Client 被完整定义，则它的数据的流向就被完整确定了.
//
// 然而, udp的转发则不一样. 一般来说, udp只handshake一次, 建立一个通道, 然后在这个通道上
// 不断申请发送到 各个远程udp地址的连接。客户端也可以选择建立多个udp通道。
type Client interface {
	ProxyCommon

	//进行 tcp承载数据的传输的握手。firstPayload 用于如 vless/trojan 这种 没有握手包的协议，可为空。
	//规定, firstPayload 由 utils.GetMTU或者 GetPacket获取, 所以写入之后可以用 utils.PutBytes 放回
	Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (wrappedConn io.ReadWriteCloser, err error)

	//建立一个通道, 然后在这个通道上 不断地申请发送到 各个远程udp地址 的连接。理论上target可为空值。
	EstablishUDPChannel(underlay net.Conn, target netLayer.Addr) (netLayer.MsgConn, error)

	//udp的拨号是否使用了多信道方式
	IsUDP_MultiChannel() bool

	//获取/拨号 一个可用的内层mux
	GetClientInnerMuxSession(wrc io.ReadWriteCloser) *smux.Session
	InnerMuxEstablished() bool
	CloseInnerMuxSession()
}

// Server 用于监听 客户端 的连接.
// 服务端是一种 “泛目标”代理，所以我们Handshake要返回 客户端请求的目标地址
// 一个 Server 掌握从最底层的tcp等到最上层的 代理协议间的所有机制。
// 一旦一个 Server 被完整定义，则它的数据的流向就被完整确定了.
type Server interface {
	ProxyCommon

	//ReadWriteCloser 为请求地址为tcp的情况, net.PacketConn 为 请求 建立的udp通道
	Handshake(underlay net.Conn) (net.Conn, netLayer.MsgConn, netLayer.Addr, error)

	//获取/监听 一个可用的内层mux
	GetServerInnerMuxSession(wlc io.ReadWriteCloser) *smux.Session
}

// FullName 可以完整表示 一个 代理的 VSI 层级.
// 这里认为, tcp/udp/kcp/raw_socket 是FirstName，具体的协议名称是 LastName, 中间层是 MiddleName。
//
// An Example of a full name:  tcp+tls+ws+vless.
// 总之，类似【域名】的规则，只不过分隔符从 点号 变成了加号。
func GetFullName(pc ProxyCommon) string {
	if n := pc.Name(); n == "direct" {
		return n
	} else {

		return getFullNameBuilder(pc, n).String()
	}
}

func getFullNameBuilder(pc ProxyCommon, n string) *strings.Builder {

	var sb strings.Builder
	sb.WriteString(pc.Network())
	sb.WriteString(pc.MiddleName())
	sb.WriteString(n)

	if i, innerProxyName := pc.HasInnerMux(); i == 2 {
		sb.WriteString("+smux+")
		sb.WriteString(innerProxyName)

	}

	return &sb

}

// return GetFullName(pc) + "://" + pc.AddrStr()
func GetVSI_url(pc ProxyCommon) string {
	n := pc.Name()
	if n == "direct" {
		return "direct://"
	}
	sb := getFullNameBuilder(pc, n)
	sb.WriteString("://")
	sb.WriteString(pc.AddrStr())

	return sb.String()
}

// 给一个节点 提供 VSI中 第 5-7层 的支持, server和 client通用. 个别方法只能用于某一端.
//
// 一个 ProxyCommon 会内嵌proxy以及上面各层的所有信息;
type ProxyCommon interface {
	Name() string       //代理协议名称, 如vless
	MiddleName() string //其它VSI层 所使用的协议，前后被加了加号，如 +tls+ws+

	Stop()

	getCommon() *ProxyCommonStruct

	/////////////////// 网络层/传输层 ///////////////////

	GetSockopt() *netLayer.Sockopt

	// 地址,若tcp/udp的话则为 ip:port/host:port的形式, 若是 unix domain socket 则是文件路径 ，
	// 在 inServer就是监听地址，在 outClient就是拨号地址
	AddrStr() string
	SetAddrStr(string)
	Network() string

	CantRoute() bool //for inServer
	GetTag() string

	/////////////////// TLS层 ///////////////////

	SetUseTLS()
	IsUseTLS() bool

	GetTLS_Server() *tlsLayer.Server
	GetTLS_Client() *tlsLayer.Client

	/////////////////// http 层 ///////////////////

	HasHeader() *httpLayer.HeaderPreset

	//默认回落地址.
	GetFallback() *netLayer.Addr

	CanFallback() bool //如果能fallback，则handshake失败后，可能会专门返回 FallbackErr,如监测到返回了 FallbackErr, 则main函数会进行 回落处理.

	Path() string

	/////////////////// 高级层 ///////////////////

	AdvancedLayer() string //如果使用了ws或者grpc，这个要返回 ws 或 grpc

	GetAdvClient() advLayer.Client
	GetAdvServer() advLayer.Server

	//IsGrpcClientMultiMode() bool

	/////////////////// 内层mux层 ///////////////////

	// 判断是否有内层mux。
	//0 为不会有 innermux, 1 为有可能有 innermux, 2 为总是使用 innerMux;
	// 规定是，客户端 只能返回0或者2， 服务端 只能返回 0或者1（除非服务端协议不支持不mux的情况，此时可以返回2）。
	// string 为 innermux内部的 代理 协议 名称。（一般用simplesocks）
	HasInnerMux() (int, string)
}

// ProxyCommonStruct 实现 ProxyCommon中除了Name 之外的其他方法.
// 规定，所有的proxy都要内嵌本struct. 我们用这种方式实现 "继承".
// 这是verysimple的架构所要求的。
// verysimple规定，在加载完配置文件后，一个listen和一个dial所使用的全部层级都是确定了的.
//  因为所有使用的层级都是确定的，就可以进行针对性优化
type ProxyCommonStruct struct {
	listenConf *ListenConf
	dialConf   *DialConf

	Addr    string
	TLS     bool
	Tag     string //可用于路由, 见 netLayer.route.go
	network string

	Sockopt *netLayer.Sockopt

	tls_s *tlsLayer.Server
	tls_c *tlsLayer.Client

	header *httpLayer.HeaderPreset

	PATH string

	cantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	advC advLayer.Client
	advS advLayer.Server

	//grpc_multi   bool

	FallbackAddr *netLayer.Addr

	innermux *smux.Session //用于存储 client的已拨号的mux连接

}

func (pcs *ProxyCommonStruct) getCommon() *ProxyCommonStruct {
	return pcs
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

func (pcs *ProxyCommonStruct) HasHeader() *httpLayer.HeaderPreset {
	return pcs.header
}

func (pcs *ProxyCommonStruct) setHeader(h *httpLayer.HeaderPreset) {
	pcs.header = h
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
	if pcs.header != nil {
		if pcs.AdvancedL != "ws" {
			sb.WriteString("+http")
		}
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

func (pcs *ProxyCommonStruct) InnerMuxEstablished() bool {
	return pcs.innermux != nil && !pcs.innermux.IsClosed()
}

//placeholder
func (pcs *ProxyCommonStruct) HasInnerMux() (int, string) {
	return 0, ""
}

func (*ProxyCommonStruct) GetServerInnerMuxSession(wlc io.ReadWriteCloser) *smux.Session {
	smuxConfig := smux.DefaultConfig()
	smuxSession, err := smux.Server(wlc, smuxConfig)
	if err != nil {
		if ce := utils.CanLogErr("smux.Server call failed"); ce != nil {
			ce.Write(
				zap.Error(err),
			)
		}
		return nil
	}
	return smuxSession
}

func (pcs *ProxyCommonStruct) CloseInnerMuxSession() {
	if pcs.innermux != nil && !pcs.innermux.IsClosed() {
		pcs.innermux.Close()
		pcs.innermux = nil
	}
}

func (pcs *ProxyCommonStruct) GetClientInnerMuxSession(wrc io.ReadWriteCloser) *smux.Session {
	if pcs.innermux != nil && !pcs.innermux.IsClosed() {
		return pcs.innermux
	} else {
		smuxConfig := smux.DefaultConfig()
		smuxSession, err := smux.Client(wrc, smuxConfig)
		if err != nil {
			if ce := utils.CanLogErr("smux.Client call failed"); ce != nil {
				ce.Write(
					zap.Error(err),
				)
			}
			return nil
		}
		pcs.innermux = smuxSession
		return smuxSession
	}
}

//return false. As a placeholder.
func (pcs *ProxyCommonStruct) IsUDP_MultiChannel() bool {
	return false
}

func (pcs *ProxyCommonStruct) GetTag() string {
	return pcs.Tag
}
func (pcs *ProxyCommonStruct) GetSockopt() *netLayer.Sockopt {
	return pcs.Sockopt
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

//try close inner mux
func (s *ProxyCommonStruct) Stop() {
	if s.innermux != nil {
		s.innermux.Close()
	}
}

//return false. As a placeholder.
func (s *ProxyCommonStruct) CanFallback() bool {
	return false
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

//func (s *ProxyCommonStruct) IsGrpcClientMultiMode() bool {
//	return s.grpc_multi
//}

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

func (s *ProxyCommonStruct) setListenConf(lc *ListenConf) {
	s.listenConf = lc
}
func (s *ProxyCommonStruct) setDialConf(dc *DialConf) {
	s.dialConf = dc
}
func (s *ProxyCommonStruct) GetAdvClient() advLayer.Client {
	return s.advC
}
func (s *ProxyCommonStruct) GetAdvServer() advLayer.Server {
	return s.advS
}

func (s *ProxyCommonStruct) InitAdvLayer() {
	switch s.AdvancedL {
	case "":
		return
	case "quic":
		s.setNetwork("udp")
	}

	creator := advLayer.ProtocolsMap[s.AdvancedL]
	if creator == nil {
		utils.Error("InitAdvLayer failed, 2, " + s.AdvancedL)

		return
	}

	ad, err := netLayer.NewAddr(s.Addr)
	if err != nil {
		utils.Error("InitAdvLayer failed, 3")

		return
	}

	if dc := s.dialConf; dc != nil {

		var Headers map[string][]string
		if dc.HttpHeader != nil {
			if dc.HttpHeader.Request != nil {
				Headers = dc.HttpHeader.Request.Headers
			}
		}

		advClient, err := creator.NewClientFromConf(&advLayer.Conf{
			Path:    dc.Path,
			Host:    dc.Host,
			IsEarly: dc.IsEarly,
			Addr:    ad,
			Headers: Headers,
			TlsConf: &tls.Config{
				InsecureSkipVerify: dc.Insecure,
				NextProtos:         dc.Alpn,
				ServerName:         dc.Host,
			},
			Extra: dc.Extra,
		})
		if err != nil {
			utils.Error("InitAdvLayer failed, 4")

			return
		}
		s.advC = advClient

	}

	if lc := s.listenConf; lc != nil {

		var Headers map[string][]string
		if lc.HttpHeader != nil {
			if lc.HttpHeader.Request != nil {
				Headers = lc.HttpHeader.Response.Headers
			}
		}

		var certArray []tls.Certificate

		if lc.TLSCert != "" && lc.TLSKey != "" {
			certArray, err = tlsLayer.GetCertArrayFromFile(lc.TLSCert, lc.TLSKey)

			if err != nil {

				if ce := utils.CanLogErr("can't create tls cert"); ce != nil {
					ce.Write(zap.String("cert", lc.TLSCert), zap.String("key", lc.TLSKey), zap.Error(err))
				}

				return
			}

		}

		advSer, err := creator.NewServerFromConf(&advLayer.Conf{
			Path:    lc.Path,
			Host:    lc.Host,
			IsEarly: lc.IsEarly,
			Addr:    ad,
			Headers: Headers,
			TlsConf: &tls.Config{
				InsecureSkipVerify: lc.Insecure,
				NextProtos:         lc.Alpn,
				ServerName:         lc.Host,
				Certificates:       certArray,
			},
			Extra: lc.Extra,
		})
		if err != nil {
			return
		}

		s.advS = advSer
	}
}
