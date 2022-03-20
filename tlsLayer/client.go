package tlsLayer

import (
	"crypto/tls"
	"log"
	"net"
	"unsafe"

	"github.com/hahahrfool/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
)

type Client struct {
	tlsConfig *tls.Config
	useTls    bool
}

func NewTlsClient(host string, insecure bool, useTls bool) *Client {

	c := &Client{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
		},
		useTls: useTls,
	}

	if useTls && utils.CanLogInfo() {
		log.Println("using utls and Chrome fingerprint for", host)
	}

	return c
}

func (c *Client) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {

	if c.useTls {
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
