/*
Package tlsLayer provides facilities for tls, including uTls, sniffing and random certificate.

Sniffing can be a part of Tls Lazy Encrypt tech.
*/
package tlsLayer

import (
	"crypto/tls"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

type Conf struct {
	Host     string
	CertConf *CertConf

	Insecure bool
	Use_uTls bool //only client
	AlpnList []string
	Minver   uint16
}

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

func GetTlsConfig(mustHasCert bool, conf Conf) *tls.Config {
	var certArray []tls.Certificate
	var err error

	if conf.CertConf != nil || mustHasCert {

		if conf.CertConf != nil {
			certArray, err = GetCertArrayFromFile(conf.CertConf.CertFile, conf.CertConf.KeyFile)

		} else {
			certArray, err = GetCertArrayFromFile("", "")

		}

		if err != nil {

			if ce := utils.CanLogErr("Can't create tls cert"); ce != nil {
				ce.Write(zap.String("cert", conf.CertConf.CertFile), zap.String("key", conf.CertConf.KeyFile), zap.Error(err))
			}

			certArray = nil

		}

	}

	tConf := &tls.Config{
		InsecureSkipVerify: conf.Insecure,
		NextProtos:         conf.AlpnList,
		ServerName:         conf.Host,
		Certificates:       certArray,
		MinVersion:         conf.Minver,
	}
	if conf.CertConf != nil && conf.CertConf.CA != "" {
		certPool, err := LoadCA(conf.CertConf.CA)
		if err != nil {
			if ce := utils.CanLogErr("Failed in loading CA"); ce != nil {
				ce.Write(zap.Error(err))
			}
		} else {
			tConf.ClientCAs = certPool
			tConf.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}
	return tConf
}

func GetUTlsConfig(conf Conf) utls.Config {
	var certArray []utls.Certificate

	if conf.CertConf != nil {
		tlscertArray, err := GetCertArrayFromFile(conf.CertConf.CertFile, conf.CertConf.KeyFile)

		if err != nil {
			if ce := utils.CanLogErr("Failed in loading client cert file"); ce != nil {
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
		InsecureSkipVerify: conf.Insecure,
		NextProtos:         conf.AlpnList,
		ServerName:         conf.Host,
		Certificates:       certArray,
		MinVersion:         conf.Minver,
	}
	if conf.CertConf != nil && conf.CertConf.CA != "" {
		certPool, err := LoadCA(conf.CertConf.CA)
		if err != nil {
			if ce := utils.CanLogErr("Err, load CA"); ce != nil {
				ce.Write(zap.Error(err))
			}
		} else {
			tConf.ClientCAs = certPool
			tConf.ClientAuth = utls.RequireAndVerifyClientCert
		}
	}
	return tConf
}
