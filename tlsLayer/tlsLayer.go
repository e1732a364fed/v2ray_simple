/*
Package tlsLayer provides facilities for tls, including sniffing.
*/
package tlsLayer

import (
	"crypto/tls"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

func GetTlsConfig(insecure, mustHasCert bool, alpn []string, host string, certConf *CertConf) *tls.Config {
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

func GetUTlsConfig(insecure bool, alpn []string, host string, certConf *CertConf) utls.Config {
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
