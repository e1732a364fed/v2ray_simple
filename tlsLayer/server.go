package tlsLayer

import (
	"crypto/tls"
	"net"
	"sync"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type Server struct {
	tlsConfig *tls.Config

	isShadow bool
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
		isShadow:  conf.Tls_type == shadowTls_t,
	}

	return s, nil
}

func (s *Server) Handshake(clientConn net.Conn) (tlsConn *Conn, err error) {
	if s.isShadow {
		var fakeConn net.Conn
		fakeConn, err = net.Dial("tcp", s.tlsConfig.ServerName+":443")
		if err != nil {
			if ce := utils.CanLogErr("Failed shadowTls server fake dial server "); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
		if ce := utils.CanLogDebug("shadowTls ready to fake "); ce != nil {
			ce.Write()
		}

		var wg sync.WaitGroup
		var e1, e2 error
		wg.Add(2)
		go func() {
			e1 = copyTls12Handshake(true, fakeConn, clientConn)
			wg.Done()

			if ce := utils.CanLogDebug("shadowTls copy client end"); ce != nil {
				ce.Write(zap.Error(e1))
			}
		}()
		go func() {
			e2 = copyTls12Handshake(false, clientConn, fakeConn)
			wg.Done()

			if ce := utils.CanLogDebug("shadowTls copy server end"); ce != nil {
				ce.Write(
					zap.Error(e2),
				)
			}
		}()

		wg.Wait()

		if e1 != nil || e2 != nil {
			e := utils.Errs{}
			e.Add(utils.ErrsItem{Index: 1, E: e1})
			e.Add(utils.ErrsItem{Index: 2, E: e2})
			return nil, e
		}

		if ce := utils.CanLogDebug("shadowTls fake ok "); ce != nil {
			ce.Write()
		}

		tlsConn = &Conn{
			Conn: clientConn,
		}

		return
	}

	rawTlsConn := tls.Server(clientConn, s.tlsConfig)
	err = rawTlsConn.Handshake()
	if err != nil {
		err = utils.ErrInErr{ErrDesc: "Failed in Tls handshake", ErrDetail: err}

		return
	}

	tlsConn = &Conn{
		Conn: rawTlsConn,
		ptr:  unsafe.Pointer(rawTlsConn),
	}

	return

}
