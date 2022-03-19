package proxy

import (
	"log"
	"net/url"
	"strings"

	"github.com/hahahrfool/v2ray_simple/config"
	"github.com/hahahrfool/v2ray_simple/utils"
)

type ClientCreator interface {
	//程序从某种配置文件格式中读取出 config.DialConf, 然后多出的配置部分以map形式给出
	NewClient(*config.DialConf) (Client, error)
	NewClientFromURL(url *url.URL) (Client, error)
}

type ServerCreator interface {
	//程序从某种配置文件格式中读取出 config.ListenConf, 然后多出的配置部分以map形式给出
	NewServer(*config.ListenConf) (Server, error)
	NewServerFromURL(url *url.URL) (Server, error)
}

var (
	clientCreatorMap = make(map[string]ClientCreator)

	serverCreatorMap = make(map[string]ServerCreator)
)

// RegisterClient is used to register a client.
// 规定，每个 实现Client的包必须使用本函数进行注册
func RegisterClient(name string, c ClientCreator) {
	clientCreatorMap[name] = c
}

// RegisterServer is used to register a proxy server
// 规定，每个 实现 Server 的包必须使用本函数进行注册
func RegisterServer(name string, c ServerCreator) {
	serverCreatorMap[name] = c
}

func NewClient(dc *config.DialConf) (Client, error) {
	protocol := dc.Protocol
	creator, ok := clientCreatorMap[protocol]
	if ok {
		return creator.NewClient(dc)
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

			return c, err

		}
	}
	return nil, utils.NewDataErr("unknown client scheme '", nil, protocol)

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
		return creator.NewClientFromURL(u)
	} else {

		//尝试判断是否套tls, 比如vlesss实际上是vless+tls，https实际上是http+tls

		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			c, err := creator.NewClientFromURL(u)
			if err != nil {
				return c, err
			}

			c.SetUseTLS()
			prepareTLS_forProxyCommon_withURL(u, true, c)

			return c, err

		}

	}

	return nil, utils.NewDataErr("unknown client scheme '", nil, u.Scheme)
}

func NewServer(lc *config.ListenConf) (Server, error) {
	protocol := lc.Protocol
	creator, ok := serverCreatorMap[protocol]
	if ok {
		ser, err := creator.NewServer(lc)
		if err != nil {
			return nil, err
		}
		configCommonForServer(ser, lc)

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

	return nil, utils.NewDataErr("unknown server scheme '", nil, protocol)
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

	return nil, utils.NewDataErr("unknown server scheme '", nil, u.Scheme)
}

//setTag, setCantRoute
func configCommonURLQueryForServer(ser ProxyCommon, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("noroute") != "" {
		nr = true
	}
	ser.setCantRoute(nr)
	ser.setTag(u.Fragment)

}

//setTag, setCantRoute
func configCommonForServer(ser ProxyCommon, lc *config.ListenConf) {
	ser.setTag(lc.Tag)
	ser.setCantRoute(lc.NoRoute)

}
