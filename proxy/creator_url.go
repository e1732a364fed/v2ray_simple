package proxy

import (
	"log"
	"net/url"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

// ClientFromURL calls the registered creator to create client. The returned bool is true if has err.
func ClientFromURL(s string) (Client, bool, utils.ErrInErr) {
	u, err := url.Parse(s)
	if err != nil {

		return nil, true, utils.ErrInErr{ErrDesc: "Can't parse client url", ErrDetail: err, Data: s}
	}

	schemeName := strings.ToLower(u.Scheme)

	creator, ok := clientCreatorMap[schemeName]
	if ok {
		return clientFromURL(creator, u, false)
	} else {

		//尝试判断是否套tls, 比如vlesss实际上是vless+tls，https实际上是http+tls

		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, ok = clientCreatorMap[realScheme]
		if ok {
			return clientFromURL(creator, u, true)
		}

	}

	return nil, false, utils.ErrInErr{ErrDesc: "Unknown client protocol ", Data: u.Scheme}
}

func clientFromURL(creator ClientCreator, u *url.URL, knownTLS bool) (Client, bool, utils.ErrInErr) {
	c, e := creator.NewClientFromURL(u)
	if e != nil {
		return nil, true, utils.ErrInErr{ErrDesc: "Err, creator.NewClientFromURL", ErrDetail: e}
	}
	configCommonByURL(c, u)

	cc := c.GetBase()

	if cc != nil {
		cc.Tag = u.Fragment

		if knownTLS {
			cc.TLS = true
			prepareTLS_forProxyCommon_withURL(u, true, c)
		}
	}

	return c, false, utils.ErrInErr{}
}

// ServerFromURL calls the registered creator to create proxy servers.
func ServerFromURL(s string) (Server, bool, utils.ErrInErr) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, true, utils.ErrInErr{
			ErrDesc:   "Can't parse server url ",
			ErrDetail: err,
			Data:      s,
		}
	}

	schemeName := strings.ToLower(u.Scheme)
	creator, ok := serverCreatorMap[schemeName]
	if ok {
		return serverFromURL(creator, u, false)

	} else {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, ok = serverCreatorMap[realScheme]
		if ok {
			return serverFromURL(creator, u, true)

		}
	}

	return nil, true, utils.ErrInErr{ErrDesc: "Unknown server protocol ", Data: u.Scheme}
}

func serverFromURL(creator ServerCreator, u *url.URL, knownTLS bool) (Server, bool, utils.ErrInErr) {
	server, err := creator.NewServerFromURL(u)
	if err != nil {
		return nil, true, utils.ErrInErr{
			ErrDesc:   "Err, creator.NewServerFromURL",
			ErrDetail: err,
		}
	}
	configCommonURLQueryForServer(server, u)

	if knownTLS {
		server.GetBase().TLS = true
		prepareTLS_forProxyCommon_withURL(u, false, server)

	}
	return server, false, utils.ErrInErr{}
}

//set Tag, cantRoute,FallbackAddr, call configCommonByURL
func configCommonURLQueryForServer(ser BaseInterface, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("noroute") != "" {
		nr = true
	}
	configCommonByURL(ser, u)

	serc := ser.GetBase()
	if serc == nil {
		return
	}
	serc.IsCantRoute = nr
	serc.Tag = u.Fragment

	fallbackStr := q.Get("fallback")

	if fallbackStr != "" {
		fa, err := netLayer.NewAddr(fallbackStr)

		if err != nil {
			if utils.ZapLogger != nil {
				utils.ZapLogger.Fatal("Failed, configCommonURLQueryForServer", zap.String("Invalid fallback", fallbackStr))
			} else {
				log.Fatalf("Invalid fallback %s\n", fallbackStr)

			}
		}

		serc.FallbackAddr = &fa
	}
}

//SetAddrStr, setNetwork, Isfullcone
func configCommonByURL(baseI BaseInterface, u *url.URL) {
	if u.Scheme != DirectName {
		baseI.SetAddrStr(u.Host) //若不给出port，那就只有host名，这样不好，我们 默认 配置里肯定给了port

	}
	base := baseI.GetBase()
	if base == nil {
		return
	}
	base.setNetwork(u.Query().Get("network"))

	base.IsFullcone = GetFullconeFromUrl(u)

}

func GetFullconeFromUrl(url *url.URL) bool {
	nStr := url.Query().Get("fullcone")
	return nStr == "true" || nStr == "1"
}
