package tlsLayer

import (
	"errors"
	"io/ioutil"
	mathrand "math/rand"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"

	"github.com/biter777/countries"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var ErrCAFileWrong = errors.New("ca file is somehow wrong")

type CertConf struct {
	CA                string
	CertFile, KeyFile string
}

func LoadCA(caFile string) (cp *x509.CertPool, err error) {
	if caFile == "" {
		err = utils.ErrNilParameter
		return
	}
	cp = x509.NewCertPool()
	data, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	if !cp.AppendCertsFromPEM(data) {
		return nil, ErrCAFileWrong
	}
	return
}

//使用 ecc p256 方式生成证书
func GenerateRandomeCert_Key() (certPEM []byte, keyPEM []byte) {

	//可参考 https://blog.ideawand.com/2017/11/22/build-certificate-that-support-Subject-Alternative-Name-SAN/

	clist := countries.All()
	country := clist[mathrand.Intn(len(clist))]

	companyName := utils.GetRandomWord()

	if ce := utils.CanLogInfo("generate random cert with"); ce != nil {
		ce.Write(zap.String("country", country.Info().Name), zap.String("company", companyName))
	}

	subject := pkix.Name{
		Country:            []string{country.Alpha2()},
		Province:           []string{country.Capital().String()},
		Organization:       []string{companyName},
		OrganizationalUnit: []string{""},
		CommonName:         "www." + companyName + ".com",
	}

	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, max)
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		//IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}

	b, err := x509.MarshalECPrivateKey(rootKey)
	if err != nil {
		panic(err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &rootKey.PublicKey, rootKey)
	if err != nil {
		panic(err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	return

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

		certFile = utils.GetFilePath(certFile)
		keyFile = utils.GetFilePath(keyFile)

		cert, err := tls.LoadX509KeyPair(utils.GetFilePath(certFile), utils.GetFilePath(keyFile))
		if err != nil {

			if ce := utils.CanLogErr("GetCertArrayFromFile failed, will use generated random cert in memory"); ce != nil {
				ce.Write(zap.Error(err))
			}

			certArray = GenerateRandomTLSCert()
			err = nil

		} else {
			certArray = []tls.Certificate{cert}

		}
	} else {
		if ce := utils.CanLogDebug("GetCertArrayFromFile generating random cert in memory"); ce != nil {
			ce.Write()
		}
		certArray = GenerateRandomTLSCert()
	}

	return
}
