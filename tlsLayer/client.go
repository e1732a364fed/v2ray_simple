package tlsLayer

import (
	"crypto/tls"
	"log"
	"net"
	"unsafe"

	"github.com/hahahrfool/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
)

// 关于utls的简单分析，可参考
//https://github.com/hahahrfool/v2ray_simple/discussions/7

type Client struct {
	tlsConfig *tls.Config
	use_uTls  bool
}

func NewClient(host string, insecure bool, use_uTls bool) *Client {

	c := &Client{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
		},
		use_uTls: use_uTls,
	}

	if use_uTls && utils.CanLogInfo() {
		log.Println("using utls and Chrome fingerprint for", host)
	}

	return c
}

func (c *Client) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {

	if c.use_uTls {
		utlsConn := utls.UClient(underlay, &utls.Config{
			InsecureSkipVerify: c.tlsConfig.InsecureSkipVerify,
			ServerName:         c.tlsConfig.ServerName,
		}, utls.HelloChrome_Auto)

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
