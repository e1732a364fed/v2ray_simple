package proxy

import (
	"net"
	"net/url"
	"strconv"
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
		var dc *DialConf

		if UrlFormat == UrlStandardFormat {
			dc = &DialConf{}
			setConfByStandardURL(&dc.CommonConf, u)

			if okTls {
				dc.TLS = true
				setTLS_forConf_withStandardURL(u, &dc.CommonConf, nil, dc)

			}
		}
		var e error
		dc, e = creator.URLToDialConf(u, dc, UrlFormat)

		if e != nil {
			return nil, e
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

		var sConf *ListenConf

		if UrlFormat == UrlStandardFormat {
			sConf = &ListenConf{}

			confQueryForServer(sConf, u)

			if okTls {
				sConf.TLS = true

				setTLS_forConf_withStandardURL(u, &sConf.CommonConf, sConf, nil)

			}

		}

		sConf, err := creator.URLToListenConf(u, sConf, UrlFormat)
		if err != nil {
			return nil, utils.ErrInErr{
				ErrDesc:   "URLToListenConf err ",
				ErrDetail: err,
				Data:      s,
			}
		}

		return newServer(creator, sConf, false)

	}

	return nil, utils.ErrInErr{ErrDesc: "Unknown server protocol ", Data: u.Scheme}
}

func getFullconeFromUrl(url *url.URL) bool {
	nStr := url.Query().Get("fullcone")
	return nStr == "true" || nStr == "1"
}

//SetAddrStr, setNetwork, Isfullcone
func setConfByStandardURL(conf *CommonConf, u *url.URL) error {
	if u.Scheme != DirectName {

		hn := u.Hostname()

		ip := net.ParseIP(hn)
		if ip != nil {
			conf.IP = hn
		} else {
			conf.Host = hn
		}

		if hn != u.Host { //给出了port
			colon := strings.LastIndexByte(u.Host, ':')
			p, err := strconv.Atoi(u.Host[colon+1:])
			if err != nil {
				return err
			} else if p < 0 || p > 65535 {
				return utils.ErrInvalidData
			}
			conf.Port = p

		}
	}
	conf.Network = u.Query().Get("network")

	conf.Fullcone = getFullconeFromUrl(u)
	conf.Tag = u.Fragment

	return nil
}

//set Tag, NoRoute,Fallback, call setConfByStandardURL
func confQueryForServer(conf *ListenConf, u *url.URL) {
	nr := false
	q := u.Query()
	if q.Get("noroute") != "" {
		nr = true
	}
	setConfByStandardURL(&conf.CommonConf, u)

	conf.NoRoute = nr

	conf.Tag = u.Fragment

	fallbackStr := q.Get("fallback")

	conf.Fallback = fallbackStr
}

//给 ProxyCommon 的tls做一些配置上的准备，从url读取配置
func setTLS_forConf_withStandardURL(u *url.URL, com *CommonConf, lc *ListenConf, dc *DialConf) error {
	q := u.Query()
	insecureStr := q.Get("insecure")
	if insecureStr != "" && insecureStr != "false" && insecureStr != "0" {

		com.Insecure = true
	}

	if dc != nil {
		utlsStr := q.Get("utls")
		useUtls := utlsStr != "" && utlsStr != "false" && utlsStr != "0"

		dc.Utls = useUtls

	} else {
		certFile := q.Get("cert")
		keyFile := q.Get("key")

		lc.TLSCert = certFile
		lc.TLSKey = keyFile

	}
	return nil
}
