/*
Package tlsLayer provides facilities for tls, including uTls,shadowTls, sniffing and random certificate.

Sniffing can be a part of Tls Lazy Encrypt tech.
*/
package tlsLayer

import (
	"crypto/tls"
	"crypto/x509"
	"strings"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

const (
	Tls_t = iota
	UTls_t
	ShadowTls_t
	ShadowTls2_t
)

func StrToType(str string) int {
	str = strings.ToLower(str)
	switch str {
	default:
		fallthrough
	case "", "go", "tls", "gotls":
		return Tls_t
	case "utls":
		return UTls_t
	case "shadow", "shadowtls", "shadowtls1", "shadowtlsv1", "shadowtls_v1", "shadowtls v1":
		return ShadowTls_t
	case "shadow2", "shadowtls2", "shadowtlsv2", "shadowtls_v2", "shadowtls v2":
		return ShadowTls2_t
	}
}

func TypeToStr(t int) string {
	switch t {
	default:
		fallthrough
	case Tls_t:
		return "tls"
	case UTls_t:
		return "utls"
	case ShadowTls_t:
		return "shadowtls_v1"
	case ShadowTls2_t:
		return "shadowtls_v2"
	}
}

type Conf struct {
	Host     string
	Insecure bool
	Minver   uint16
	Maxver   uint16
	AlpnList []string
	CertConf *CertConf

	Tls_type int

	RejectUnknownSni bool //only server
	CipherSuites     []uint16

	Extra map[string]any //用于shadowTls
}

func (tConf Conf) IsShadowTls() bool {
	return tConf.Tls_type == ShadowTls2_t || tConf.Tls_type == ShadowTls_t
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
