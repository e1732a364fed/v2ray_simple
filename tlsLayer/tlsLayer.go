/*
Package tlsLayer provides support for tlsLayer, including sniffing.
*/
package tlsLayer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

func GenerateRandomeCert_Key() ([]byte, []byte) {
	//ecc p256

	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, max)
	subject := pkix.Name{
		Country:            []string{"ZZ"},
		Province:           []string{"asfdsdaf"},
		Organization:       []string{"daffd"},
		OrganizationalUnit: []string{"adsadf"},
		CommonName:         "127.0.0.1",
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	b, err := x509.MarshalECPrivateKey(rootKey)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &rootKey.PublicKey, rootKey)
	if err != nil {
		panic(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return certPEM, keyPEM

	/*
		//rsa

		pk, _ := rsa.GenerateKey(rand.Reader, 2048)

		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})

	*/
}

func GenerateRandomTLSCert() []tls.Certificate {

	tlsCert, err := tls.X509KeyPair(GenerateRandomeCert_Key())
	if err != nil {
		panic(err)
	}
	return []tls.Certificate{tlsCert}

}

func GenerateRandomCertKeyFiles(cfn, kfn string) {

	cb, kb := GenerateRandomeCert_Key()

	certOut, err := os.Create(cfn)
	if err != nil {
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal("failed to open file", zap.Error(err))
		} else {
			log.Fatalf("failed to open file %s", err)

		}
	}

	certOut.Write(cb)

	kOut, err := os.Create(kfn)
	if err != nil {
		if utils.ZapLogger != nil {
			utils.ZapLogger.Fatal("failed to open file", zap.Error(err))
		} else {
			log.Fatalf("failed to open file %s", err)
		}
	}

	kOut.Write(kb)

	certOut.Close()
	kOut.Close()

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
