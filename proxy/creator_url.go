package proxy

import (
	"net/url"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	UrlNativeFormat   = iota //proxy对应的标准文档所定义的url模式，一般散布于对应github的文档上
	UrlStandardFormat        //VS定义的 供 所有proxy 使用的 标准 url模式

)

var (
	// Url格式 设置以何种方式解析 命令行模式/极简模式 中出现的url配置
	//
	//关于url格式的详细， 见 https://github.com/e1732a364fed/v2ray_simple/discussions/163
	UrlFormat = UrlStandardFormat
)

// ClientFromURL calls the registered creator to create client. The returned bool is true if has err.
func ClientFromURL(s string) (Client, error) {
	u, err := url.Parse(s)
	if err != nil {

		return nil, utils.ErrInErr{ErrDesc: "Can't parse client url", ErrDetail: err, Data: s}
	}

	schemeName := strings.ToLower(u.Scheme)

	creator, ok := clientCreatorMap[schemeName]

	var okTls bool

	if !ok {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, okTls = clientCreatorMap[realScheme]
	}
	//尝试判断是否套tls, 比如vlesss实际上是vless+tls，https实际上是http+tls

	if okTls {
		ok = true
	}

	if ok {

		dc, e := creator.URLToDialConf(u, UrlStandardFormat)
		if e != nil {

			return nil, e
		}
		if UrlFormat == UrlStandardFormat {
			setConfByStandardURL(&dc.CommonConf, u)

			if okTls {
				dc.TLS = true
				prepareTLS_forConf_withStandardURL(u, &dc.CommonConf, nil, dc)

			}
		}

		c, e := newClient(creator, dc, false)

		if e != nil {

			return nil, e
		}

		return c, nil
	}

	return nil, utils.ErrInErr{ErrDesc: "Unknown client protocol ", Data: u.Scheme}
}

// ServerFromURL calls the registered creator to create proxy servers.
func ServerFromURL(s string) (Server, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, utils.ErrInErr{
			ErrDesc:   "Can't parse server url ",
			ErrDetail: err,
			Data:      s,
		}
	}

	schemeName := strings.ToLower(u.Scheme)
	creator, ok := serverCreatorMap[schemeName]

	var okTls bool

	if !ok {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creator, okTls = serverCreatorMap[realScheme]

	}

	if okTls {
		ok = true
	}

	if ok {

		sConf, err := creator.URLToListenConf(u, UrlFormat)
		if err != nil {
			return nil, utils.ErrInErr{
				ErrDesc:   "URLToListenConf err ",
				ErrDetail: err,
				Data:      s,
			}
		}

		if UrlFormat == UrlStandardFormat {
			confQueryForServer(sConf, u)

			if okTls {
				sConf.TLS = true

				prepareTLS_forConf_withStandardURL(u, &sConf.CommonConf, sConf, nil)

			}

		}

		return newServer(creator, sConf, false)

	}

	return nil, utils.ErrInErr{ErrDesc: "Unknown server protocol ", Data: u.Scheme}
}

/*
//set Tag, cantRoute,FallbackAddr, call configCommonByURL
func configCommonURLQueryForServer(ser BaseInterface, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("noroute") != "" {
		nr = true
	}
	configCommonByStandardURL(ser, u)

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
*/

/*
//SetAddrStr, setNetwork, Isfullcone
func configCommonByStandardURL(baseI BaseInterface, u *url.URL) {
	if u.Scheme != DirectName {
		baseI.SetAddrStr(u.Host) //若不给出port，那就只有host名，这样不好，我们 默认 配置里肯定给了port

	}
	base := baseI.GetBase()
	if base == nil {
		return
	}
	base.setNetwork(u.Query().Get("network"))

	base.IsFullcone = getFullconeFromUrl(u)

}
*/

func getFullconeFromUrl(url *url.URL) bool {
	nStr := url.Query().Get("fullcone")
	return nStr == "true" || nStr == "1"
}
