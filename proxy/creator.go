package proxy

import (
	"net"
	"net/url"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	serverCreatorMap = map[string]ServerCreator{
		RejectName: RejectCreator{},
	}
	clientCreatorMap = map[string]ClientCreator{
		DirectName: DirectCreator{},
		RejectName: RejectCreator{},
	}
)

func PrintAllServerNames() {
	utils.PrintStr("===============================\nSupported Proxy Listen protocols:\n")
	for _, v := range utils.GetMapSortedKeySlice(serverCreatorMap) {
		utils.PrintStr(v)
		utils.PrintStr("\n")
	}
}

func PrintAllClientNames() {
	utils.PrintStr("===============================\nSupported Proxy Dial protocols:\n")

	for _, v := range utils.GetMapSortedKeySlice(clientCreatorMap) {
		utils.PrintStr(v)
		utils.PrintStr("\n")
	}
}

type CreatorCommonStruct struct{}

func (CreatorCommonStruct) MultiTransportLayer() bool {
	return false
}
func (CreatorCommonStruct) AfterCommonConfServer(Server) {}

type CreatorCommon interface {
	//若为true，则表明该协议可同时使用tcp和udp来传输数据。direct, socks5 和 shadowsocks 都为true。
	//此时，是否开启udp取决于Network(), 如果为dual, 则均支持; 如果仅为tcp或者udp，则不支持。
	// direct的默认Network为dual。
	MultiTransportLayer() bool
}

// 可通过标准配置或url 来初始化。
type ClientCreator interface {
	CreatorCommon
	//大部分通用内容都会被proxy包解析，方法只需要处理proxy包未知的内容
	NewClient(*DialConf) (Client, error) //标准配置

	//URLToDialConf 执行proxy自定义的非标准代码;
	//iv: initial value, can be nil.
	URLToDialConf(url *url.URL, iv *DialConf, format int) (*DialConf, error)
	//DialConfToURL(url *DialConf, format int) (*url.URL, error)

	UseUDPAsMsgConn() bool //默认情况下，UDP会被包装成流式协议，除非指出需要利用udp的固定包长度特性; direct 和 shadowsocks会返回true.

}

// 可通过标准配置或url 来初始化。
type ServerCreator interface {
	CreatorCommon

	NewServer(*ListenConf) (Server, error)
	AfterCommonConfServer(Server)

	URLToListenConf(url *url.URL, iv *ListenConf, format int) (*ListenConf, error)
	//ListenConfToURL(url *ListenConf, format int) (*url.URL, error)
}

// 规定，每个 实现Client的包必须使用本函数进行注册。
// direct 和 reject 统一使用本包提供的方法, 自定义协议不得覆盖 direct 和 reject。
func RegisterClient(name string, c ClientCreator) {
	switch name {
	case DirectName, RejectName:
		return
	}
	clientCreatorMap[name] = c

}

// 规定，每个 实现 Server 的包必须使用本函数进行注册
func RegisterServer(name string, c ServerCreator) {
	serverCreatorMap[name] = c
}

func NewClient(dc *DialConf) (Client, error) {
	protocol := dc.Protocol
	creator, ok := clientCreatorMap[protocol]
	if ok {

		return newClient(creator, dc, false)
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			return newClient(creator, dc, true)
		}
	}
	return nil, utils.ErrInErr{ErrDesc: "Unknown client protocol ", Data: protocol}

}

func newClient(creator ClientCreator, dc *DialConf, knownTls bool) (Client, error) {
	c, e := creator.NewClient(dc)
	if e != nil {
		return nil, e
	}
	e = configCommonForClient(c, dc)
	if e != nil {
		return nil, e
	}
	if dc.TLS || knownTls {
		c.GetBase().TLS = true
		e = prepareTLS_forClient(c, dc)
	}
	if dc.SendThrough != "" {
		setSendThroughByNetwork(c.GetBase(), dc.SendThrough)
	}
	return c, e

}

func setSendThroughByNetwork(b *Base, throughStr string) error {

	st, err := netLayer.StrToNetAddr(netLayer.DualNetworkName, throughStr)
	if err != nil {
		return utils.ErrInErr{ErrDesc: "parse sendthrough addr failed", ErrDetail: err}
	}

	switch b.Network() {
	case netLayer.DualNetworkName:

		b.LTA = st.(*netLayer.TCPUDPAddr).TCPAddr
		b.LUA = st.(*netLayer.TCPUDPAddr).UDPAddr
	case "tcp":
		b.LTA = st.(*net.TCPAddr)
	case "udp":
		b.LUA = st.(*net.UDPAddr)
	}

	return nil
}

// SetAddrStr,  ConfigCommon
func configCommonForClient(cli BaseInterface, dc *DialConf) error {
	if cli.Name() != DirectName {
		cli.SetAddrStr(dc.GetAddrStrForListenOrDial())
	}

	clic := cli.GetBase()
	if clic == nil {
		return nil
	}

	clic.DialConf = dc

	clic.ConfigCommon(&dc.CommonConf)

	return nil
}

func NewServer(lc *ListenConf) (Server, error) {
	protocol := lc.Protocol
	creator, ok := serverCreatorMap[protocol]
	if ok {
		return newServer(creator, lc, false)
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = serverCreatorMap[realScheme]
		if ok {
			return newServer(creator, lc, true)
		}
	}

	return nil, utils.ErrInErr{ErrDesc: "Unknown server protocol ", Data: protocol}
}

func newServer(creator ServerCreator, lc *ListenConf, knownTls bool) (Server, error) {
	ser, err := creator.NewServer(lc)
	if err != nil {
		return nil, err
	}
	err = configCommonForServer(ser, lc)
	if err != nil {
		return nil, err
	}

	if lc.TLS || knownTls {
		ser.GetBase().TLS = true
		err = prepareTLS_forServer(ser, lc)
		if err != nil {
			return nil, utils.ErrInErr{ErrDesc: "Failed, prepareTLS", ErrDetail: err}

		}
	}
	creator.AfterCommonConfServer(ser)
	return ser, nil

}

// SetAddrStr, setCantRoute,setFallback, ConfigCommon
func configCommonForServer(ser BaseInterface, lc *ListenConf) error {
	ser.SetAddrStr(lc.GetAddrStrForListenOrDial())
	serc := ser.GetBase()
	if serc == nil {
		return nil
	}
	serc.ListenConf = lc
	serc.IsCantRoute = lc.NoRoute

	serc.ConfigCommon(&lc.CommonConf)

	if fallbackThing := lc.Fallback; fallbackThing != nil {
		fa, err := netLayer.NewAddrFromAny(fallbackThing)

		if err != nil {
			return utils.ErrInErr{ErrDesc: "Failed, configCommonURLQueryForServer", Data: fallbackThing}

		}

		serc.FallbackAddr = &fa
	}

	return nil
}
