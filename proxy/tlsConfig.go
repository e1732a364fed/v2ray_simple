package proxy

import (
	"crypto/tls"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
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

// use dc.Host, dc.Insecure, dc.Utls, dc.Alpn.
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

	conf := tlsLayer.Conf{
		Host:     dc.Host,
		Insecure: dc.Insecure,
		//Use_uTls:     dc.Utls,
		Tls_type:     tlsLayer.StrToType(dc.TlsType),
		AlpnList:     alpnList,
		CertConf:     certConf,
		Minver:       getTlsMinVerFromExtra(dc.Extra),
		Maxver:       getTlsMaxVerFromExtra(dc.Extra),
		CipherSuites: getTlsCipherSuitesFromExtra(dc.Extra),
	}

	clic.Tls_c = tlsLayer.NewClient(conf)
	return nil
}

// use lc.Host, lc.TLSCert, lc.TLSKey, lc.Insecure, lc.Alpn, lc.Extra
func prepareTLS_forServer(com BaseInterface, lc *ListenConf) error {

	serc := com.GetBase()
	if serc == nil {
		return nil
	}

	alpnList := updateAlpnListByAdvLayer(com, lc.Alpn)

	conf := tlsLayer.Conf{
		Host: lc.Host,
		CertConf: &tlsLayer.CertConf{
			CertFile: lc.TLSCert, KeyFile: lc.TLSKey, CA: lc.CA,
		},
		Tls_type: tlsLayer.StrToType(lc.TlsType),

		Insecure: lc.Insecure,
		AlpnList: alpnList,
		Minver:   getTlsMinVerFromExtra(lc.Extra),
		Maxver:   getTlsMaxVerFromExtra(lc.Extra),

		RejectUnknownSni: getTlsRejectUnknownSniFromExtra(lc.Extra),
		CipherSuites:     getTlsCipherSuitesFromExtra(lc.Extra),
	}

	tlsserver, err := tlsLayer.NewServer(conf)

	if err == nil {
		serc.Tls_s = tlsserver
		serc.TlsConf = conf
	} else {
		return err
	}
	return nil
}

func getTlsMinVerFromExtra(extra map[string]any) uint16 {
	if len(extra) > 0 {
		if thing := extra["tls_minVersion"]; thing != nil {
			if str, ok := (thing).(string); ok && len(str) > 0 {
				switch str {
				case "1.2":
					return tls.VersionTLS12
				}
			}
		}
	}

	return tls.VersionTLS13
}

func getTlsMaxVerFromExtra(extra map[string]any) uint16 {

	fromStr := func(str string) uint16 {
		switch str {
		case "1.2":
			return tls.VersionTLS12
		case "1.3":
			return tls.VersionTLS13
		default:
			if ce := utils.CanLogErr("parse tls version failed"); ce != nil {
				ce.Write(zap.String("given", str))
			}
			return tls.VersionTLS13
		}
	}

	if len(extra) > 0 {
		if thing := extra["tls_maxVersion"]; thing != nil {
			if str, ok := (thing).(string); ok && len(str) > 0 {
				return fromStr(str)
			}
		}
	}

	return tls.VersionTLS13
}

func getTlsRejectUnknownSniFromExtra(extra map[string]any) bool {
	if len(extra) > 0 {
		if thing := extra["rejectUnknownSni"]; thing != nil {
			if is, ok := utils.AnyToBool(thing); ok && is {
				return true
			}
		}
	}

	return false
}

func getTlsCipherSuitesFromExtra(extra map[string]any) []uint16 {
	if len(extra) > 0 {
		if thing := extra["tls_cipherSuites"]; thing != nil {
			if is, ok := utils.AnyToUInt16Array(thing); ok && len(is) > 0 {
				return is
			}
			if strs, ok := thing.([]string); ok {
				var v []uint16
				for _, s := range strs {
					cs := tlsLayer.StrToCipherSuite(s)
					if cs > 0 {
						v = append(v, cs)
					}
				}
				if len(v) > 0 {
					return v
				}
			}
		}
	}

	return nil
}
