package proxy

import (
	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
)

func updateAlpnListByAdvLayer(com BaseInterface, alpnList []string) (result []string) {
	result = alpnList

	common := com.GetBase()
	if common == nil {
		return
	}

	if adv := com.AdvancedLayer(); adv != "" {
		var creator advLayer.Creator

		if c := common.AdvC; c != nil {
			creator = c
		} else if s := common.AdvS; s != nil {
			creator = s
		} else {
			return
		}

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

	return
}

//use dc.Host, dc.Insecure, dc.Utls, dc.Alpn.
func prepareTLS_forClient(com BaseInterface, dc *DialConf) error {
	alpnList := updateAlpnListByAdvLayer(com, dc.Alpn)

	clic := com.GetBase()
	if clic == nil {
		return nil
	}

	var certConf *tlsLayer.CertConf
	if dc.TLSCert != "" && dc.TLSKey != "" {
		certConf = &tlsLayer.CertConf{
			CertFile: dc.TLSCert,
			KeyFile:  dc.TLSKey,
		}
	}
	var minVer uint16 = tlsLayer.GetMinVerFromExtra(dc.Extra)

	clic.Tls_c = tlsLayer.NewClient(dc.Host, dc.Insecure, dc.Utls, alpnList, certConf, minVer)
	return nil
}

//use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, lc.Alpn.
func prepareTLS_forServer(com BaseInterface, lc *ListenConf) error {

	serc := com.GetBase()
	if serc == nil {
		return nil
	}

	alpnList := updateAlpnListByAdvLayer(com, lc.Alpn)

	var minVer uint16 = tlsLayer.GetMinVerFromExtra(lc.Extra)

	tlsserver, err := tlsLayer.NewServer(lc.Host, &tlsLayer.CertConf{
		CertFile: lc.TLSCert, KeyFile: lc.TLSKey, CA: lc.CA,
	}, lc.Insecure, alpnList, minVer)

	if err == nil {
		serc.Tls_s = tlsserver
	} else {
		return err
	}
	return nil
}
