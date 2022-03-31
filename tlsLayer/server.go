package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"

	"github.com/hahahrfool/v2ray_simple/utils"
)

type Server struct {
	tlsConfig *tls.Config
}

//如 certFile, keyFile 有一项没给出，则会自动生成随机证书
func NewServer(host, certFile, keyFile string, isInsecure bool, alpnList []string) (*Server, error) {

	certArray, err := GetCertArrayFromFile(certFile, keyFile)

	if err != nil {
		return nil, err
	}

	s := &Server{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: isInsecure,
			ServerName:         host,
			Certificates:       certArray,
			NextProtos:         alpnList,
		},
	}

	return s, nil
}

func (s *Server) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {
	rawTlsConn := tls.Server(underlay, s.tlsConfig)
	err = rawTlsConn.Handshake()
	if err != nil {
		err = utils.ErrInErr{ErrDesc: "tlsLayer: tls握手失败", ErrDetail: err}

		return
	}

	tlsConn = &Conn{
		Conn: rawTlsConn,
		ptr:  unsafe.Pointer(rawTlsConn),
	}

	return

}
