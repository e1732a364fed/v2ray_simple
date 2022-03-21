package proxy

import (
	"log"
	"net/url"
	"strings"

	"github.com/hahahrfool/v2ray_simple/utils"
)

type ClientCreator interface {
	//程序从某种配置文件格式中读取出 DialConf
	NewClient(*DialConf) (Client, error)
	NewClientFromURL(url *url.URL) (Client, error)
}

type ServerCreator interface {
	//程序从某种配置文件格式中读取出 ListenConf
	NewServer(*ListenConf) (Server, error)
	NewServerFromURL(url *url.URL) (Server, error)
}

var (
	clientCreatorMap = make(map[string]ClientCreator)
	serverCreatorMap = make(map[string]ServerCreator)
)

// 规定，每个 实现Client的包必须使用本函数进行注册
func RegisterClient(name string, c ClientCreator) {
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
		c, e := creator.NewClient(dc)
		if e != nil {
			return nil, e
		}
		configCommonForClient(c, dc)
		if dc.TLS {
			c.SetUseTLS()
			e = prepareTLS_forClient(c, dc)
			configCommonForClient(c, dc)
			return c, e
		}

		return c, nil
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			c, err := creator.NewClient(dc)
			if err != nil {
				return c, err
			}

			c.SetUseTLS()
			err = prepareTLS_forClient(c, dc)
			configCommonForClient(c, dc)
			return c, err

		}
	}
	return nil, utils.NewDataErr("unknown client protocol '", nil, protocol)

}

// ClientFromURL calls the registered creator to create client.
// dialer is the default upstream dialer so cannot be nil, we can use Default when calling this function.
func ClientFromURL(s string) (Client, error) {
	u, err := url.Parse(s)
	if err != nil {

		return nil, utils.NewDataErr("can not parse client url", err, s)
	}

	schemeName := strings.ToLower(u.Scheme)

	creator, ok := clientCreatorMap[schemeName]
	if ok {
		c, e := creator.NewClientFromURL(u)
		if e != nil {
			return nil, e
		}
		configCommonByURL(c, u)
		return c, nil
	} else {

		//尝试判断是否套tls, 比如vlesss实际上是vless+tls，https实际上是http+tls

		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			c, err := creator.NewClientFromURL(u)
			if err != nil {
				return c, err
			}
			configCommonByURL(c, u)

			c.SetUseTLS()
			prepareTLS_forProxyCommon_withURL(u, true, c)

			return c, err

		}

	}

	return nil, utils.NewDataErr("unknown client protocol '", nil, u.Scheme)
}

func NewServer(lc *ListenConf) (Server, error) {
	protocol := lc.Protocol
	creator, ok := serverCreatorMap[protocol]
	if ok {
		ser, err := creator.NewServer(lc)
		if err != nil {
			return nil, err
		}
		configCommonForServer(ser, lc)

		if lc.TLS {
			ser.SetUseTLS()
			err = prepareTLS_forServer(ser, lc)
			if err != nil {
				log.Fatalln("prepareTLS error", err)
			}
			configCommonForServer(ser, lc)
			return ser, nil
		}

		return ser, nil
	} else {
		realScheme := strings.TrimSuffix(protocol, "s")
		creator, ok = serverCreatorMap[realScheme]
		if ok {
			ser, err := creator.NewServer(lc)
			if err != nil {
				return nil, err
			}

			ser.SetUseTLS()
			err = prepareTLS_forServer(ser, lc)
			if err != nil {
				log.Fatalln("prepareTLS error", err)
			}
			configCommonForServer(ser, lc)
			return ser, nil

		}
	}

	return nil, utils.NewDataErr("unknown server protocol '", nil, protocol)
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
	creator, ok := serverCreatorMap[schemeName]
	if ok {
		ser, err := creator.NewServerFromURL(u)
		if err != nil {
			return nil, err
		}
		configCommonURLQueryForServer(ser, u)

		return ser, nil
	} else {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, ok = serverCreatorMap[realScheme]
		if ok {
			server, err := creator.NewServerFromURL(u)
			if err != nil {
				return nil, err
			}
			configCommonURLQueryForServer(server, u)

			server.SetUseTLS()
			prepareTLS_forProxyCommon_withURL(u, false, server)
			return server, nil

		}
	}

	return nil, utils.NewDataErr("unknown server protocol '", nil, u.Scheme)
}

//setTag, setCantRoute, call configCommonByURL
func configCommonURLQueryForServer(ser ProxyCommon, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("noroute") != "" {
		nr = true
	}

	ser.setCantRoute(nr)
	ser.setTag(u.Fragment)

	configCommonByURL(ser, u)

}

//setAdvancedLayer
func configCommonByURL(ser ProxyCommon, u *url.URL) {
	q := u.Query()
	wsStr := q.Get("ws")
	if wsStr != "" && wsStr != "0" && wsStr != "false" {
		ser.setAdvancedLayer("ws")
	}

}

//setAdvancedLayer
func configCommon(ser ProxyCommon, cc *CommonConf) {
	ser.setAdvancedLayer(cc.AdvancedLayer)

}

//setNetwork, setIsDial(true),setDialConf(dc), call  configCommon
func configCommonForClient(cli ProxyCommon, dc *DialConf) {
	cli.setNetwork(dc.Network)
	cli.setIsDial(true)
	cli.setDialConf(dc)
	cli.setTag(dc.Tag)

	configCommon(cli, &dc.CommonConf)

	if dc.AdvancedLayer == "ws" {
		cli.initWS_client()
	}
}

//setNetwork, setTag, setCantRoute,setListenConf(lc), call configCommon
func configCommonForServer(ser ProxyCommon, lc *ListenConf) {
	ser.setNetwork(lc.Network)
	ser.setListenConf(lc)
	ser.setTag(lc.Tag)
	ser.setCantRoute(lc.NoRoute)
	configCommon(ser, &lc.CommonConf)
}
