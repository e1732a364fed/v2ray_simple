package proxy

import (
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
)

//use dc.Host, dc.Insecure, dc.Utls, dc.Alpn.
func prepareTLS_forClient(com ProxyCommon, dc *DialConf) error {
	alpnList := dc.Alpn

	clic := com.getCommon()
	if clic == nil {
		return nil
	}

	switch com.AdvancedLayer() {
	case "quic":
		clic.setNetwork("udp")
		return nil
	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}
	clic.setTLS_Client(tlsLayer.NewClient(dc.Host, dc.Insecure, dc.Utls, alpnList))
	return nil
}

//use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, lc.Alpn.
func prepareTLS_forServer(com ProxyCommon, lc *ListenConf) error {
	// 这里直接不检查 字符串就直接传给 tlsLayer.NewServer
	// 所以要求 cert和 key 不在程序本身目录 的话，就要给出完整路径

	serc := com.getCommon()
	if serc == nil {
		return nil
	}

	alpnList := lc.Alpn
	switch com.AdvancedLayer() {
	case "quic":

		serc.setNetwork("udp")
		return nil

	case "grpc":
		has_h2 := false
		for _, a := range alpnList {
			if a == httpLayer.H2_Str {
				has_h2 = true
				break
			}
		}
		if !has_h2 {
			alpnList = append([]string{httpLayer.H2_Str}, alpnList...)
		}
	}

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
