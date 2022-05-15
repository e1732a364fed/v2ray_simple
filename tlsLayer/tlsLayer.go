/*
Package tlsLayer provides facilities for tls, including uTls, sniffing and random certificate.
*/
package tlsLayer

import (
	"crypto/tls"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

func GetMinVerFromExtra(extra map[string]any) uint16 {
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

func GetTlsConfig(insecure, mustHasCert bool, alpn []string, host string, certConf *CertConf, minVer uint16) *tls.Config {
	var certArray []tls.Certificate
	var err error

	if certConf != nil || mustHasCert {

		if certConf != nil {
			certArray, err = GetCertArrayFromFile(certConf.CertFile, certConf.KeyFile)

		} else {
			certArray, err = GetCertArrayFromFile("", "")

		}

		if err != nil {

			if ce := utils.CanLogErr("can't create tls cert"); ce != nil {
				ce.Write(zap.String("cert", certConf.CertFile), zap.String("key", certConf.KeyFile), zap.Error(err))
			}

			certArray = nil

		}

	}

	tConf := &tls.Config{
		InsecureSkipVerify: insecure,
		NextProtos:         alpn,
		ServerName:         host,
		Certificates:       certArray,
		MinVersion:         minVer,
	}
	if certConf != nil && certConf.CA != "" {
		certPool, err := LoadCA(certConf.CA)
		if err != nil {
			if ce := utils.CanLogErr("load CA failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		} else {
			tConf.ClientCAs = certPool
			tConf.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}
	return tConf
}

func GetUTlsConfig(insecure bool, alpn []string, host string, certConf *CertConf, minVer uint16) utls.Config {
	var certArray []utls.Certificate

	if certConf != nil {
		tlscertArray, err := GetCertArrayFromFile(certConf.CertFile, certConf.KeyFile)

		if err != nil {
			if ce := utils.CanLogErr("load client cert file failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			certArray = nil
		} else {

			for _, c := range tlscertArray {

				certArray = append(certArray, *(*utls.Certificate)(unsafe.Pointer(&c)))
			}

		}

	}

	tConf := utls.Config{
		InsecureSkipVerify: insecure,
		NextProtos:         alpn,
		ServerName:         host,
		Certificates:       certArray,
		MinVersion:         minVer,
	}
	if certConf != nil && certConf.CA != "" {
		certPool, err := LoadCA(certConf.CA)
		if err != nil {
			if ce := utils.CanLogErr("load CA failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		} else {
			tConf.ClientCAs = certPool
			tConf.ClientAuth = utls.RequireAndVerifyClientCert
		}
	}
	return tConf
}
