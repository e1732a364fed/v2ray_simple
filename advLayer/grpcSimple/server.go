package grpcSimple

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

type Server struct {
	Config

	http2.Server

	path string
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

}

func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackConnChan chan advLayer.FallbackMeta) {
	go s.Server.ServeConn(underlay, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {

			//log.Println("request headers", rq.Header)

			/*
				关于h2c

				https://pkg.go.dev/golang.org/x/net/http2/h2c#example-NewHandler

				https://github.com/thrawn01/h2c-golang-example

				https://gist.github.com/tom-code/698b20b342be7bbf6ab692884b8476d5
			*/

			if p := rq.URL.Path; p != s.path {
				if ce := utils.CanLogWarn("grpc Server got wrong path"); ce != nil {
					ce.Write(zap.String("path", p))
				}

				if fallbackConnChan != nil {

					if ce := utils.CanLogInfo("grpc will fallback"); ce != nil {
						ce.Write(zap.String("path", p))
					}

					rq2 := rq.Clone(context.Background())
					rq2.Body = nil
					rq2.ContentLength = 0
					buf := utils.GetBuf()
					rq2.Write(buf)

					sc := newServerConn(rw, rq)

					fallbackConnChan <- advLayer.FallbackMeta{Path: p, Conn: sc, FirstBuffer: buf}

					<-sc.closeChan

				} else {
					rw.WriteHeader(http.StatusNotFound)

				}

				return
			}

			if ct := rq.Header.Get("Content-Type"); ct != "application/grpc" {
				if ce := utils.CanLogWarn("GRPC Server got right path but with wrong Content-Type"); ce != nil {
					ce.Write(zap.String("type", ct), zap.String("tips", "you might need to use a more complex path"))
				}

				rw.WriteHeader(http.StatusUnsupportedMediaType)

				return
			}

			headerMap := rw.Header()
			headerMap.Add("Content-Type", "application/grpc") //necessary
			rw.WriteHeader(http.StatusOK)

			sc := newServerConn(rw, rq)
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
		g.Closer.Close()
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
