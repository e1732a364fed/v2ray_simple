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
	"math/big"
	"net"
	"os"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

//使用 ecc p256 方式生成证书
func GenerateRandomeCert_Key() ([]byte, []byte) {

	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, max)

	//可参考 https://blog.ideawand.com/2017/11/22/build-certificate-that-support-Subject-Alternative-Name-SAN/

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

// 会调用 GenerateRandomeCert_Key 来生成证书，并生成包含该证书的 []tls.Certificate
func GenerateRandomTLSCert() []tls.Certificate {

	tlsCert, err := tls.X509KeyPair(GenerateRandomeCert_Key())
	if err != nil {
		panic(err)
	}
	return []tls.Certificate{tlsCert}

}

// 会调用 GenerateRandomeCert_Key 来生成证书，并输出到文件
func GenerateRandomCertKeyFiles(cfn, kfn string) error {

	cb, kb := GenerateRandomeCert_Key()

	certOut, err := os.Create(cfn)
	if err != nil {

		return err
	}

	certOut.Write(cb)

	kOut, err := os.Create(kfn)
	if err != nil {

		return err
	}

	kOut.Write(kb)

	certOut.Close()
	kOut.Close()

	return nil
}

//若 certFile, keyFile 有一项没给出，则会自动生成随机证书
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
