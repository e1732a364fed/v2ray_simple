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

func (c *Client) Handshake(underlay net.Conn) (tlsConn *tls.Conn, err error) {
	tlsConn = tls.Client(underlay, c.tlsConfig)
	err = tlsConn.Handshake()
	return
}
