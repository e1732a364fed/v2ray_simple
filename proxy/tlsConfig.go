package proxy

import (
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
)

func updateAlpnListByAdvLayer(com ProxyCommon, alpnList []string) (result []string) {
	result = alpnList

	if adv := com.AdvancedLayer(); adv != "" {
		if creator := advLayer.ProtocolsMap[adv]; creator != nil {
			if alpn, must := creator.GetDefaultAlpn(); must {
				has_alpn := false

				for _, a := range alpnList {
					if a == alpn {
						has_alpn = true
						break
					}
				}

				if !has_alpn {
					result = append([]string{alpn}, alpnList...)
				}
			}
		}
	}

	return
}

//use dc.Host, dc.Insecure, dc.Utls, dc.Alpn.
func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	alpnList := updateAlpnListByAdvLayer(com, dc.Alpn)

	clic := com.getCommon()
	if clic == nil {
		return nil
	}

	clic.setTLS_Client(tlsLayer.NewClient(dc.Host, dc.Insecure, dc.Utls, alpnList))
	return nil
}

//use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, lc.Alpn.
func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {

	serc := com.getCommon()
	if serc == nil {
		return nil
	}

	alpnList := updateAlpnListByAdvLayer(com, lc.Alpn)

	tlsserver, err := tlsLayer.NewServer(lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, alpnList)
	if err == nil {
		serc.setTLS_Server(tlsserver)
	} else {
		return err
	}
	return nil
}

//给 ProxyCommon 的tls做一些配置上的准备，从url读取配置
func prepareTLS_forProxyCommon_withURL(u *url.URL, isclient bool, com ProxyCommon) error {
	insecureStr := u.Query().Get("insecure")
	insecure := false
	if insecureStr != "" && insecureStr != "false" && insecureStr != "0" {
		insecure = true
	}

	if isclient {
		utlsStr := u.Query().Get("utls")
		useUtls := utlsStr != "" && utlsStr != "false" && utlsStr != "0"
		com.getCommon().setTLS_Client(tlsLayer.NewClient(u.Host, insecure, useUtls, nil))

	} else {
		certFile := u.Query().Get("cert")
		keyFile := u.Query().Get("key")

		hostAndPort := u.Host
		sni, _, _ := net.SplitHostPort(hostAndPort)

		tlsserver, err := tlsLayer.NewServer(sni, certFile, keyFile, insecure, nil)
		if err == nil {
			com.getCommon().setTLS_Server(tlsserver)
		} else {
			return err
		}
	}
	return nil
}
