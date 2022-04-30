package grpcSimple

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type Server struct {
	Config

	http2.Server

	path string

	newSubConnChan   chan net.Conn
	fallbackConnChan chan advLayer.FallbackMeta

	stopOnce sync.Once

	closed bool

	underlay net.Conn
}

func (s *Server) GetPath() string {
	return s.ServiceName
}

func (*Server) IsMux() bool {
	return true
}

func (*Server) IsSuper() bool {
	return false
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.closed = true

		s.underlay.Close()

		if s.fallbackConnChan != nil {
			close(s.fallbackConnChan)
		}
		if s.newSubConnChan != nil {
			close(s.newSubConnChan)
		}
	})
}

func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackConnChan chan advLayer.FallbackMeta) {
	s.underlay = underlay
	s.fallbackConnChan = fallbackConnChan
	s.newSubConnChan = newSubConnChan

	go s.Server.ServeConn(underlay, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {
			if s.closed {
				return
			}

			//log.Println("request headers", rq.Header)
			/*
				we will try to fallback to h2c.

				about h2c

				https://pkg.go.dev/golang.org/x/net/http2/h2c#example-NewHandler
				https://github.com/thrawn01/h2c-golang-example

				test h2c:

				curl -k -v --http2-prior-knowledge https://localhost:4434/sfd

				curl -k -v --http2-prior-knowledge -X POST -F 'asdf=1234'  https://localhost:4434/sfd

			*/

			shouldFallback := false

			p := rq.URL.Path

			if p != s.path {
				if ce := utils.CanLogWarn("grpc Server got wrong path"); ce != nil {
					ce.Write(zap.String("path", p))
				}

				shouldFallback = true
			} else if ct := rq.Header.Get("Content-Type"); ct != grpcContentType {
				if ce := utils.CanLogWarn("GRPC Server got right path but with wrong Content-Type"); ce != nil {
					ce.Write(zap.String("type", ct), zap.String("tips", "you might want to use a more complex path"))
				}

				shouldFallback = true
			}

			if shouldFallback {
				if fallbackConnChan == nil {
					rw.WriteHeader(http.StatusNotFound)

				} else {

					if ce := utils.CanLogInfo("grpc will fallback"); ce != nil {
						ce.Write(
							zap.String("path", p),
							zap.String("method", rq.Method),
							zap.String("raddr", rq.RemoteAddr))
					}

					buf := utils.GetBuf()

					rq.Write(buf)
					//log.Println("ContentLength", rq.ContentLength, buf.String())

					sc := &netLayer.IOWrapper{
						Reader:         utils.DummyReadCloser{},
						Writer:         rw,
						CloseChan:      make(chan struct{}),
						FirstWriteChan: make(chan struct{}),
					}

					if s.closed {
						return
					}
					fallbackConnChan <- advLayer.FallbackMeta{Path: p, Conn: sc, FirstBuffer: buf}

					<-sc.CloseChan

				}

				return
			}

			headerMap := rw.Header()
			headerMap.Add("Content-Type", grpcContentType) //necessary
			rw.WriteHeader(http.StatusOK)

			sc := newServerConn(rw, rq)
			if s.closed {
				return
			}
			newSubConnChan <- sc
			<-sc.closeChan //necessary
		}),
	})
}

func newServerConn(rw http.ResponseWriter, rq *http.Request) (sc *ServerConn) {
	sc = &ServerConn{
		commonPart: commonPart{
			br: bufio.NewReader(rq.Body),
		},

		Writer:    rw,
		Closer:    rq.Body,
		closeChan: make(chan int),
	}

	ta, e := net.ResolveTCPAddr("tcp", rq.RemoteAddr)
	if e == nil {
		sc.ra = ta
	}

	sc.timeouter = timeouter{
		closeFunc: func() {
			sc.Close()
		},
	}
	return
}

type ServerConn struct {
	commonPart
	timeouter

	io.Closer
	io.Writer

	closeOnce sync.Once
	closeChan chan int
	closed    bool
}

func (g *ServerConn) Close() error {
	g.closeOnce.Do(func() {
		g.closed = true
		close(g.closeChan)
		if g.Closer != nil {
			g.Closer.Close()

		}
	})
	return nil
}

func (g *ServerConn) Write(b []byte) (n int, err error) {

	//the determination of g.closed is necessary, or it might panic when calling Write or Flush

	if g.closed {
		return 0, net.ErrClosed
	} else {
		buf := commonWrite(b)

		if g.closed {
			return 0, net.ErrClosed
		}
		_, err = g.Writer.Write(buf.Bytes())
		utils.PutBuf(buf)

		if err == nil && !g.closed {
			g.Writer.(http.Flusher).Flush() //necessary

		}

		return len(b), err
	}

}
