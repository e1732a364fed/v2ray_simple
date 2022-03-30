/*
Package tlsLayer 提供tls层的各种支持. 也包括tls流量的嗅探功能
*/
package tlsLayer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"

	"github.com/hahahrfool/v2ray_simple/utils"
)

func GenerateRandomTLSCert() []tls.Certificate {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return []tls.Certificate{tlsCert}
}

func GetCertArrayFromFile(certFile, keyFile string) (certArray []tls.Certificate, err error) {
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(utils.GetFilePath(certFile), utils.GetFilePath(keyFile))
		if err != nil {
			return nil, err
		}
		certArray = []tls.Certificate{cert}
	} else {
		certArray = GenerateRandomTLSCert()
	}

	return
}
