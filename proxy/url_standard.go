package proxy

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

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

			p, err := strconv.Atoi(u.Host[strings.LastIndexByte(u.Host, ':'):])
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
func prepareTLS_forConf_withStandardURL(u *url.URL, com *CommonConf, lc *ListenConf, dc *DialConf) error {
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
