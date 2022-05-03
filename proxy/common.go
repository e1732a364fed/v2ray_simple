package proxy

import (
	"crypto/tls"
	"io"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

// Provide supports for all VSI model layers except proxy layer for a proxy.
type ProxyCommon interface {
	Name() string       //代理协议名称, 如vless
	MiddleName() string //不包含传输层 和 代理层的 其它VSI层 所使用的协议，前后被加了加号，如 +tls+ws+

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

	CanFallback() bool //如果能fallback，则handshake失败后，可能会专门返回 httpLayer.FallbackErr,如监测到返回了 FallbackErr, 则main函数会进行 回落处理.

	/////////////////// 高级层 ///////////////////

	AdvancedLayer() string //所使用的高级层的协议名称

	GetAdvClient() advLayer.Client
	GetAdvServer() advLayer.Server

	/////////////////// 内层mux层 ///////////////////

	// 判断是否有内层mux。
	//0 为不会有 innermux, 1 为有可能有 innermux, 2 为总是使用 innerMux;
	// 规定是，客户端 只能返回0/2， 服务端 只能返回 0/1（除非服务端协议不支持不mux的情况，此时可以返回2）。
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

	cantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	advC advLayer.Client
	advS advLayer.Server

	FallbackAddr *netLayer.Addr

	innermux *smux.Session //用于存储 client的已拨号的mux连接

}

func (pcs *ProxyCommonStruct) getCommon() *ProxyCommonStruct {
	return pcs
}

func (pcs *ProxyCommonStruct) Network() string {
	return pcs.network
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
		utils.Error("InitAdvLayer failed, not supported, " + s.AdvancedL)

		return
	}

	ad, err := netLayer.NewAddr(s.Addr)
	if err != nil {
		if ce := utils.CanLogErr("InitAdvLayer addr failed "); ce != nil {
			ce.Write(
				zap.Error(err),
			)
		}

		return
	}

	if dc := s.dialConf; dc != nil {

		var Headers *httpLayer.HeaderPreset
		if creator.CanHandleHeaders() {
			Headers = dc.HttpHeader
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

			if ce := utils.CanLogErr("InitAdvLayer client failed "); ce != nil {
				ce.Write(
					zap.String("protocol", s.AdvancedL),
					zap.Error(err),
				)
			}

			return
		}
		s.advC = advClient

	}

	if lc := s.listenConf; lc != nil {

		var Headers *httpLayer.HeaderPreset

		if creator.CanHandleHeaders() {
			Headers = lc.HttpHeader
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

			if ce := utils.CanLogErr("InitAdvLayer server failed "); ce != nil {
				ce.Write(
					zap.String("protocol", s.AdvancedL),
					zap.Error(err),
				)
			}

			return
		}

		s.advS = advSer
	}
}
