package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/exp/slices"
)

type Server struct {
	tlsConfig *tls.Config

	tlstype int

	shadowpass string
}

// 如 certFile, keyFile 有一项没给出，则会自动生成随机证书
func NewServer(conf Conf) (*Server, error) {

	//服务端必须给出 http/1.1 等，否则不会协商出这个alpn，而我们为了回落，是需要协商出所有可能需要的 alpn的。

	//而且我们如果不提供 h1 和 h2 的alpn的话，很容易被审查者察觉的。

	if conf.AlpnList == nil {
		conf.AlpnList = []string{"http/1.1", "h2"}
	} else {

		if !slices.Contains(conf.AlpnList, "http/1.1") {
			conf.AlpnList = append(conf.AlpnList, "http/1.1")
		}
		if !slices.Contains(conf.AlpnList, "h2") {
			conf.AlpnList = append(conf.AlpnList, "h2")
		}
	}

	s := &Server{
		tlsConfig: GetTlsConfig(true, conf),
		tlstype:   conf.Tls_type,
	}

	if conf.Tls_type == ShadowTls2_t {
		s.shadowpass = getShadowTlsPasswordFromExtra(conf.Extra)

	}

	return s, nil
}

func (s *Server) Handshake(clientConn net.Conn) (tlsConn Conn, err error) {

	switch s.tlstype {
	case ShadowTls_t:
		return shadowTls1(s.tlsConfig.ServerName, clientConn)
	case ShadowTls2_t:
		return shadowTls2(s.tlsConfig.ServerName, clientConn, s.shadowpass)

	}

	rawTlsConn := tls.Server(clientConn, s.tlsConfig)
	err = rawTlsConn.Handshake()
	if err != nil {
		err = utils.ErrInErr{ErrDesc: "Failed in Tls handshake", ErrDetail: err}

		return
	}

	tlsConn = &conn{
		Conn: rawTlsConn,
		ptr:  unsafe.Pointer(rawTlsConn),
	}

	return

}
