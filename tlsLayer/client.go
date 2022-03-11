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
	rawConn := tls.Client(underlay, c.tlsConfig)
	err = rawConn.Handshake()

	if err != nil {
		return
	}

	tlsConn = &Conn{
		Conn: rawConn,
	}

	return
}
