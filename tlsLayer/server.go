package tlsLayer

import (
	"crypto/tls"
	"net"

	"github.com/hahahrfool/v2ray_simple/utils"
)

type Server struct {
	tlsConfig *tls.Config
}

func NewServer(host, certFile, keyFile string, isInsecure bool) (*Server, error) {

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	s := &Server{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: isInsecure,
			ServerName:         host,
			Certificates:       []tls.Certificate{cert},
		},
	}

	return s, nil
}

func (s *Server) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {
	rawTlsConn := tls.Server(underlay, s.tlsConfig)
	err = rawTlsConn.Handshake()
	if err != nil {
		err = utils.NewErr("tlsLayer: tls握手失败", err)

		return
	}

	tlsConn = &Conn{
		Conn: rawTlsConn,
	}

	return

}
