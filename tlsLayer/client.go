package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

// 关于utls的简单分析，可参考
//https://github.com/e1732a364fed/v2ray_simple/discussions/7

type Client struct {
	tlsConfig  *tls.Config
	uTlsConfig utls.Config
	use_uTls   bool
	alpnList   []string
}

func NewClient(host string, insecure bool, use_uTls bool, alpnList []string, certConf *CertConf) *Client {

	c := &Client{
		use_uTls: use_uTls,
	}

	c.alpnList = alpnList

	if use_uTls {
		tConf := utls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
			NextProtos:         alpnList,
		}

		if certConf != nil {
			certArray, err := GetCertArrayFromFile(certConf.CertFile, certConf.KeyFile)

			if err != nil {
				if ce := utils.CanLogErr("load client cert file failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
			} else {
				var array []utls.Certificate

				for _, c := range certArray {

					array = append(array, *(*utls.Certificate)(unsafe.Pointer(&c)))
				}

				tConf.Certificates = array
			}

		}

		c.uTlsConfig = tConf

		if ce := utils.CanLogInfo("using utls and Chrome fingerprint for"); ce != nil {
			ce.Write(zap.String("host", host))
		}
	} else {
		tConf := &tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
			NextProtos:         alpnList,
		}
		if certConf != nil {
			certArray, err := GetCertArrayFromFile(certConf.CertFile, certConf.KeyFile)

			if err != nil {
				if ce := utils.CanLogErr("load client cert file failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
			} else {
				tConf.Certificates = certArray

			}
		}
		c.tlsConfig = tConf

	}

	return c
}

func (c *Client) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {

	if c.use_uTls {
		configCopy := c.uTlsConfig //发现uTlsConfig竟然没法使用指针，握手一次后配置文件就会被污染，只能拷贝
		//否则的话接下来的握手客户端会报错： tls: CurvePreferences includes unsupported curve

		utlsConn := utls.UClient(underlay, &configCopy, utls.HelloChrome_Auto)
		err = utlsConn.Handshake()
		if err != nil {
			return
		}
		tlsConn = &Conn{
			Conn:           utlsConn,
			ptr:            unsafe.Pointer(utlsConn.Conn),
			tlsPackageType: utlsPackage,
		}

	} else {
		officialConn := tls.Client(underlay, c.tlsConfig)
		err = officialConn.Handshake()
		if err != nil {
			return
		}

		tlsConn = &Conn{
			Conn:           officialConn,
			ptr:            unsafe.Pointer(officialConn),
			tlsPackageType: official,
		}

	}
	return
}
