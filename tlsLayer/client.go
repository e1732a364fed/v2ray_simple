package tlsLayer

import (
	"crypto/tls"
	"net"
)

type Client struct {
	tlsConfig *tls.Config
}

func NewTlsClient(host string, insecure bool) *Client {

	c := &Client{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
		},
	}

	return c
}

func (c *Client) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {
	rawTlsConn := tls.Client(underlay, c.tlsConfig)
	err = rawTlsConn.Handshake()

	if err != nil {
		return
	}

	tlsConn = &Conn{
		Conn: rawTlsConn,
	}

	return
}
