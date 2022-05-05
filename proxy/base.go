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

//BaseInterface provides supports for all VSI model layers except proxy layer.
type BaseInterface interface {
	Name() string       //代理协议名称, 如vless
	MiddleName() string //不包含传输层 和 代理层的 其它VSI层 所使用的协议，前后被加了加号，如 +tls+ws+

	Stop()

	GetBase() *Base
	GetTag() string

	/////////////////// 网络层 ///////////////////

	// 地址,若tcp/udp的话则为 ip:port/host:port的形式, 若是 unix domain socket 则是文件路径 ，
	// 在 inServer就是监听地址，在 outClient就是拨号地址
	AddrStr() string
	SetAddrStr(string)

	GetSockopt() *netLayer.Sockopt

	CantRoute() bool //for inServer

	/////////////////// 传输层 ///////////////////

	Network() string //传输层协议,如 tcp, udp, unix, kcp, etc. 这里叫做Network而不是transport, 是遵循 golang 标准包 net包的用法。我们兼容 net的Listen等方法, 可把Network直接作为 net.Listen等方法的 network 参数。
	GetXver() int

	/////////////////// TLS层 ///////////////////

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

// Base 实现 BaseInterface 中除了Name 之外的其他方法.
// 规定，所有的proxy都要内嵌本struct. 我们用这种方式实现 "继承".
// 这是verysimple的架构所要求的。
// verysimple规定，在加载完配置文件后，listen/dial 所使用的全部层级都是完整确定了的.
//  因为所有使用的层级都是确定的，就可以进行针对性优化
type Base struct {
	ListenConf *ListenConf
	DialConf   *DialConf

	Addr           string
	TLS            bool
	Tag            string //可用于路由, 见 netLayer.route.go
	TransportLayer string

	Sockopt *netLayer.Sockopt
	Xver    int

	Tls_s *tlsLayer.Server
	Tls_c *tlsLayer.Client

	Header *httpLayer.HeaderPreset

	IsCantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	AdvC advLayer.Client
	AdvS advLayer.Server

	FallbackAddr *netLayer.Addr

	Innermux *smux.Session //用于存储 client的已拨号的mux连接

}

func (pcs *Base) GetBase() *Base {
	return pcs
}

func (pcs *Base) Network() string {
	return pcs.TransportLayer
}

func (pcs *Base) GetXver() int {
	return pcs.Xver
}

func (pcs *Base) HasHeader() *httpLayer.HeaderPreset {
	return pcs.Header
}

func (pcs *Base) GetFallback() *netLayer.Addr {
	return pcs.FallbackAddr
}

func (pcs *Base) MiddleName() string {
	var sb strings.Builder
	sb.WriteString("")

	if pcs.TLS {
		sb.WriteString("+tls")
	}
	if pcs.Header != nil {
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

func (pcs *Base) CantRoute() bool {
	return pcs.IsCantRoute
}

func (pcs *Base) InnerMuxEstablished() bool {
	return pcs.Innermux != nil && !pcs.Innermux.IsClosed()
}

//placeholder
func (pcs *Base) HasInnerMux() (int, string) {
	return 0, ""
}

func (*Base) GetServerInnerMuxSession(wlc io.ReadWriteCloser) *smux.Session {
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

func (pcs *Base) CloseInnerMuxSession() {
	if pcs.Innermux != nil && !pcs.Innermux.IsClosed() {
		pcs.Innermux.Close()
		pcs.Innermux = nil
	}
}

func (pcs *Base) GetClientInnerMuxSession(wrc io.ReadWriteCloser) *smux.Session {
	if pcs.Innermux != nil && !pcs.Innermux.IsClosed() {
		return pcs.Innermux
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
		pcs.Innermux = smuxSession
		return smuxSession
	}
}

//return false. As a placeholder.
func (pcs *Base) IsUDP_MultiChannel() bool {
	return false
}

func (pcs *Base) GetTag() string {
	return pcs.Tag
}
func (pcs *Base) GetSockopt() *netLayer.Sockopt {
	return pcs.Sockopt
}

func (pcs *Base) setNetwork(network string) {
	if network == "" {
		pcs.TransportLayer = "tcp"

	} else {
		pcs.TransportLayer = network

	}
}

func (pcs *Base) AdvancedLayer() string {
	return pcs.AdvancedL
}

//try close inner mux
func (s *Base) Stop() {
	if s.Innermux != nil {
		s.Innermux.Close()
	}
}

//return false. As a placeholder.
func (s *Base) CanFallback() bool {
	return false
}

func (s *Base) GetTLS_Server() *tlsLayer.Server {
	return s.Tls_s
}
func (s *Base) GetTLS_Client() *tlsLayer.Client {
	return s.Tls_c
}

func (s *Base) AddrStr() string {
	return s.Addr
}
func (s *Base) SetAddrStr(a string) {
	s.Addr = a
}

func (s *Base) IsUseTLS() bool {
	return s.TLS
}

func (s *Base) GetAdvClient() advLayer.Client {
	return s.AdvC
}
func (s *Base) GetAdvServer() advLayer.Server {
	return s.AdvS
}

//setNetwork, xver, Tag,Sockopt,header,AdvancedL, InitAdvLayer
func (c *Base) ConfigCommon(cc *CommonConf) {

	c.setNetwork(cc.Network)
	c.Xver = cc.Xver
	c.Tag = cc.Tag
	c.Sockopt = cc.Sockopt

	if cc.HttpHeader != nil {
		cc.HttpHeader.AssignDefaultValue()
		c.Header = (cc.HttpHeader)
	}

	c.AdvancedL = cc.AdvancedLayer

	c.InitAdvLayer()
}

func (s *Base) InitAdvLayer() {
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

	if dc := s.DialConf; dc != nil {

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
			Xver:    dc.Xver,
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
		s.AdvC = advClient

	}

	if lc := s.ListenConf; lc != nil {

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
			Xver:    lc.Xver,
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

		s.AdvS = advSer
	}
}
