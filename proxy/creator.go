package proxy

import (
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

//可通过两种配置方式来初始化。
type ClientCreator interface {
	NewClient(*DialConf) (Client, error)
	NewClientFromURL(url *url.URL) (Client, error)
}

//可通过两种配置方式来初始化。
type ServerCreator interface {
	NewServer(*ListenConf) (Server, error)
	NewServerFromURL(url *url.URL) (Server, error)
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

		return newclient(creator, dc, false)
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			return newclient(creator, dc, true)
		}
	}
	return nil, utils.ErrInErr{ErrDesc: "Unknown client protocol ", Data: protocol}

}

func newclient(creator ClientCreator, dc *DialConf, knownTls bool) (Client, error) {
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
	return c, e

}

//SetAddrStr,  ConfigCommon
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
		return newserver(creator, lc, false)
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = serverCreatorMap[realScheme]
		if ok {
			return newserver(creator, lc, true)
		}
	}

	return nil, utils.ErrInErr{ErrDesc: "Unknown server protocol ", Data: protocol}
}

func newserver(creator ServerCreator, lc *ListenConf, knownTls bool) (Server, error) {
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
	return ser, nil

}

//SetAddrStr, setCantRoute,setFallback, ConfigCommon
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
