/*
Package tlsLayer provides facilities for tls, including uTls, sniffing and random certificate.

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

func rejectUnknownGetCertificateFunc(certs []*tls.Certificate) func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if len(certs) == 0 {
			return nil, utils.ErrInErr{ErrDesc: "len(certs) == 0", ErrDetail: utils.ErrInvalidData}
		}
		if hello == nil {
			return nil, utils.ErrInErr{ErrDesc: "hello==nil", ErrDetail: utils.ErrInvalidData}
		}
		sni := strings.ToLower(hello.ServerName)

		gsni := "*"
		if index := strings.IndexByte(sni, '.'); index != -1 {
			gsni += sni[index:]
		}
		for _, cert := range certs {
			if cert.Leaf == nil {
				var e error
				cert.Leaf, e = x509.ParseCertificate(cert.Certificate[0])
				if e != nil {
					return nil, utils.ErrInErr{ErrDesc: "rejectUnknown: x509.ParseCertificate failed ", ErrDetail: e}
				}
			}

			if cert.Leaf.Subject.CommonName == sni || cert.Leaf.Subject.CommonName == gsni {
				return cert, nil
			}
			for _, name := range cert.Leaf.DNSNames {
				if name == sni || name == gsni {
					return cert, nil
				}
			}
		}
		return nil, utils.ErrInErr{ErrDesc: "rejectUnknownSNI", ErrDetail: utils.ErrInvalidData, Data: sni}
	}
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
