/*
Package tlsLayer provides facilities for tls, including uTls, sniffing and random certificate.

Sniffing can be a part of Tls Lazy Encrypt tech.
*/
package tlsLayer

import (
	"crypto/tls"
	"crypto/x509"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

type Conf struct {
	Host     string
	Insecure bool
	Minver   uint16
	Maxver   uint16
	AlpnList []string
	CertConf *CertConf

	Use_uTls         bool //only client
	RejectUnknownSni bool //only server
	CipherSuites     []uint16
}

func GetTlsConfig(mustHasCert bool, conf Conf) *tls.Config {
	var certArray []tls.Certificate
	var err error
	var randcert bool

	if conf.CertConf != nil || mustHasCert {

		if conf.CertConf != nil {
			certArray, err = GetCertArrayFromFile(conf.CertConf.CertFile, conf.CertConf.KeyFile)

		} else {
			certArray, err = GetCertArrayFromFile("", "")
			randcert = true

		}

		if err != nil {

			if ce := utils.CanLogErr("Can't init tls cert"); ce != nil {
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
		MaxVersion:         conf.Maxver,
		CipherSuites:       conf.CipherSuites,
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
	if conf.RejectUnknownSni {
		tConf.GetCertificate = rejectUnknownGetCertificateFunc(utils.ArrayToPtrArray(certArray))
	}
	if randcert && conf.Host == "" {
		x, _ := x509.ParseCertificate(certArray[0].Certificate[0])
		tConf.ServerName = x.Subject.CommonName
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
		MaxVersion:         conf.Maxver,
		CipherSuites:       conf.CipherSuites,
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
