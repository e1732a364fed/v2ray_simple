package proxy

import (
	"crypto/tls"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

// BaseInterface provides supports for all VSI model layers except proxy layer.
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

	Sniffing() bool //for inServer, 是否开启嗅探功能

	/////////////////// TLS层 ///////////////////

	IsUseTLS() bool
	IsLazyTls() bool

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
//
//	因为所有使用的层级都是确定的，就可以进行针对性优化
type Base struct {
	ListenConf *ListenConf
	DialConf   *DialConf

	Addr           string
	TLS            bool
	Tag            string //可用于路由, 见 netLayer.route.go
	TransportLayer string

	Sockopt *netLayer.Sockopt
	Xver    int

	IsFullcone bool

	Tls_s *tlsLayer.Server
	Tls_c *tlsLayer.Client

	Header *httpLayer.HeaderPreset

	IsCantRoute bool //for inServer, 若为true，则 inServer 读得的数据 不会经过分流，一定会通过用户指定的remoteclient发出

	AdvancedL string

	AdvC advLayer.Client
	AdvS advLayer.Server

	FallbackAddr *netLayer.Addr

	Innermux *smux.Session //用于存储 client的已拨号的mux连接

	sync.Mutex

	//用于sendthrough
	LTA *net.TCPAddr
	LUA *net.UDPAddr
}

func (b *Base) GetBase() *Base {
	return b
}

func (b *Base) LocalTCPAddr() *net.TCPAddr {
	return b.LTA
}
func (b *Base) LocalUDPAddr() *net.UDPAddr {
	return (b.LUA)
}

func (b *Base) Network() string {
	return b.TransportLayer
}

func (b *Base) GetXver() int {
	return b.Xver
}

func (b *Base) HasHeader() *httpLayer.HeaderPreset {
	return b.Header
}

func (b *Base) GetFallback() *netLayer.Addr {
	return b.FallbackAddr
}

func (b *Base) MiddleName() string {
	var sb strings.Builder
	sb.WriteString("")

	if b.TLS {
		sb.WriteString("+tls")
		if b.IsLazyTls() {
			sb.WriteString("+lazy")
		}
	}
	advL := b.AdvancedL
	if b.Header != nil {
		if advL != "ws" && advL != "grpc" {
			sb.WriteString("+http")
		}
	}
	if advL != "" {
		sb.WriteString("+")
		sb.WriteString(advL)
	}
	sb.WriteString("+")
	return sb.String()
}

func (b *Base) CantRoute() bool {
	return b.IsCantRoute
}

func (b *Base) Sniffing() bool {

	if b.ListenConf == nil {
		return false
	}
	if b.ListenConf.SniffConf == nil {
		return false
	}
	return b.ListenConf.SniffConf.Enable
}

func (b *Base) InnerMuxEstablished() bool {

	return b.Innermux != nil && !b.Innermux.IsClosed()
}

// placeholder
func (b *Base) HasInnerMux() (int, string) {
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

func (b *Base) CloseInnerMuxSession() {
	if b.InnerMuxEstablished() {
		b.Innermux.Close()
		b.Innermux = nil
	}
}

func (b *Base) GetClientInnerMuxSession(wrc io.ReadWriteCloser) *smux.Session {
	if b.InnerMuxEstablished() {
		return b.Innermux
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
		b.Innermux = smuxSession
		return smuxSession
	}
}

// return false. As a placeholder.
func (b *Base) IsUDP_MultiChannel() bool {
	return false
}

func (b *Base) GetTag() string {
	return b.Tag
}
func (b *Base) GetSockopt() *netLayer.Sockopt {
	return b.Sockopt
}

func (b *Base) setNetwork(network string) {

	b.TransportLayer = network

}

func (b *Base) AdvancedLayer() string {
	return b.AdvancedL
}

// try close inner mux and stop AdvS
func (b *Base) Stop() {
	if b.Innermux != nil {
		b.Innermux.Close()
	}

	if b.AdvS != nil {
		b.AdvS.Stop()
	}
}

// return false. As a placeholder.
func (b *Base) CanFallback() bool {
	return false
}

func (b *Base) GetTLS_Server() *tlsLayer.Server {
	return b.Tls_s
}
func (b *Base) GetTLS_Client() *tlsLayer.Client {
	return b.Tls_c
}

func (b *Base) AddrStr() string {
	return b.Addr
}
func (b *Base) SetAddrStr(a string) {
	b.Addr = a
}

func (b *Base) IsUseTLS() bool {
	return b.TLS
}

func (b *Base) IsLazyTls() bool {
	if b.DialConf != nil {
		return b.DialConf.Lazy
	}

	if b.ListenConf != nil {
		return b.ListenConf.Lazy
	}
	return false
}

func (b *Base) GetAdvClient() advLayer.Client {
	return b.AdvC
}
func (b *Base) GetAdvServer() advLayer.Server {
	return b.AdvS
}

// setNetwork, Xver, Tag,Sockopt, IsFullcone, Header,AdvancedL, InitAdvLayer
func (b *Base) ConfigCommon(cc *CommonConf) {

	b.setNetwork(cc.Network)
	b.Xver = cc.Xver
	b.Tag = cc.Tag
	b.Sockopt = cc.Sockopt
	b.IsFullcone = cc.Fullcone

	if cc.HttpHeader != nil {
		cc.HttpHeader.AssignDefaultValue()
		b.Header = (cc.HttpHeader)
	}

	b.AdvancedL = cc.AdvancedLayer

	b.InitAdvLayer()
}

// 高级层就像代理层一样重要，可以注册多种包，配置选项也比较多。
func (b *Base) InitAdvLayer() {
	switch b.AdvancedL {
	case "":
		return
	case "quic":
		b.setNetwork("udp")
	}

	creator := advLayer.ProtocolsMap[b.AdvancedL]
	if creator == nil {
		utils.Error("InitAdvLayer failed, not supported, " + b.AdvancedL)

		return
	}

	ad, err := netLayer.NewAddr(b.Addr)
	if err != nil {
		if ce := utils.CanLogErr("InitAdvLayer addr failed "); ce != nil {
			ce.Write(
				zap.Error(err),
			)
		}

		return
	}
	ad.Network = b.Network()

	if dc := b.DialConf; dc != nil {

		var Headers *httpLayer.HeaderPreset
		if creator.CanHandleHeaders() {
			Headers = dc.HttpHeader
		}

		var tConf *tls.Config
		if creator.IsSuper() {

			var certConf *tlsLayer.CertConf

			if dc.TLSCert != "" && dc.TLSKey != "" {
				certConf = &tlsLayer.CertConf{
					CertFile: dc.TLSCert,
					KeyFile:  dc.TLSKey,
				}
			}

			tConf = tlsLayer.GetTlsConfig(false, tlsLayer.Conf{
				Insecure:     dc.Insecure,
				AlpnList:     dc.Alpn,
				Host:         dc.Host,
				CertConf:     certConf,
				Minver:       getTlsMinVerFromExtra(dc.Extra),
				Maxver:       getTlsMaxVerFromExtra(dc.Extra),
				CipherSuites: getTlsCipherSuitesFromExtra(dc.Extra),
			})

		}

		aConf := &advLayer.Conf{
			Path:    dc.Path,
			Host:    dc.Host,
			IsEarly: dc.IsEarly,
			Addr:    ad,
			Headers: Headers,
			Xver:    dc.Xver,
			TlsConf: tConf,
			Extra:   dc.Extra,
		}

		advClient, err := creator.NewClientFromConf(aConf)
		if err != nil {

			if ce := utils.CanLogErr("Failed in InitAdvLayer client"); ce != nil {
				ce.Write(
					zap.String("protocol", b.AdvancedL),
					zap.Error(err),
				)
			}

			return
		}
		b.AdvC = advClient

	}

	if lc := b.ListenConf; lc != nil {

		var Headers *httpLayer.HeaderPreset

		if creator.CanHandleHeaders() {
			Headers = lc.HttpHeader
		}

		aConf := &advLayer.Conf{
			Path:    lc.Path,
			Host:    lc.Host,
			IsEarly: lc.IsEarly,
			Xver:    lc.Xver,
			Addr:    ad,
			Headers: Headers,
			Extra:   lc.Extra,
		}

		if creator.IsSuper() {

			aConf.TlsConf = tlsLayer.GetTlsConfig(true, tlsLayer.Conf{
				Insecure: lc.Insecure,
				AlpnList: lc.Alpn,
				Host:     lc.Host,
				CertConf: &tlsLayer.CertConf{
					CertFile: lc.TLSCert, KeyFile: lc.TLSKey, CA: lc.CA,
				},
				Minver:       getTlsMinVerFromExtra(lc.Extra),
				Maxver:       getTlsMaxVerFromExtra(lc.Extra),
				CipherSuites: getTlsCipherSuitesFromExtra(lc.Extra),
			})
		}

		advSer, err := creator.NewServerFromConf(aConf)
		if err != nil {

			if ce := utils.CanLogErr("Failed in InitAdvLayer server"); ce != nil {
				ce.Write(
					zap.String("protocol", b.AdvancedL),
					zap.Error(err),
				)
			}

			return
		}

		b.AdvS = advSer
	}
}

func (d *Base) DialTCP(target netLayer.Addr) (result net.Conn, err error) {
	if d.Sockopt != nil {
		if d.LTA == nil {
			result, err = target.DialWithOpt(d.Sockopt, nil) //避免把nil的 *net.TCPAddr 装箱到 net.Addr里

		} else {
			result, err = target.DialWithOpt(d.Sockopt, d.LTA)

		}
	} else {
		if d.LTA == nil {
			result, err = target.Dial(nil, nil)

		} else {
			result, err = target.Dial(nil, d.LTA)

		}
	}
	return
}

func (d *Base) DialUDP(target netLayer.Addr) (mc *netLayer.UDPMsgConn, err error) {

	mc, err = netLayer.NewUDPMsgConn(d.LUA, d.IsFullcone, false, d.Sockopt)
	return

}
func (d *Base) SelfListen() (is bool, tcp, udp int) {
	return
}
