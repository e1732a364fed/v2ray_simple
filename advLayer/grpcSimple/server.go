package grpcSimple

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
)

// implements advLayer.MuxServer
type Server struct {
	Creator

	Config

	http2.Server

	Headers *httpLayer.HeaderPreset

	path string

	newSubConnChan   chan net.Conn
	fallbackConnChan chan httpLayer.FallbackMeta

	stopOnce sync.Once

	closed bool

	underlay net.Conn //目前仅用于Close
}

func (s *Server) GetPath() string {
	return s.ServiceName
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		s.closed = true

		if s.underlay != nil {
			s.underlay.Close()
		}

		if s.fallbackConnChan != nil {
			close(s.fallbackConnChan)
		}
		if s.newSubConnChan != nil {
			close(s.newSubConnChan)
		}
	})
}

var (
	clientPreface = []byte(http2.ClientPreface)
)

func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackConnChan chan httpLayer.FallbackMeta) {
	s.underlay = underlay
	s.fallbackConnChan = fallbackConnChan
	s.newSubConnChan = newSubConnChan

	//先过滤一下h2c 的 preface. 因为不是h2c的话，依然可以试图回落到 h1.

	//可以参考 golang.org/x/net/http2/server.go 里的 readPreface 方法.

	bs := utils.GetPacket()
	proxy.SetCommonReadTimeout(underlay)

	var notH2c bool
	n, err := underlay.Read(bs)
	if err != nil {
		if ce := utils.CanLogDebug("grpc try read preface failed"); ce != nil {
			ce.Write()
		}

		return

	} else if n < len(clientPreface) || !bytes.Equal(bs[:len(clientPreface)], clientPreface) {
		notH2c = true
	}

	netLayer.PersistConn(underlay)

	firstBuf := bytes.NewBuffer(bs[:n])

	if notH2c {
		if ce := utils.CanLogInfo("Grpc got not h2c request"); ce != nil {
			ce.Write()
		}

		if fallbackConnChan != nil {
			_, method, path, _, failreason := httpLayer.ParseH1Request(bs, false)
			if failreason != 0 {

				fallbackConnChan <- httpLayer.FallbackMeta{
					Conn:         underlay,
					H1RequestBuf: firstBuf,
				}
			} else {
				fallbackConnChan <- httpLayer.FallbackMeta{
					Path:         path,
					Method:       method,
					Conn:         underlay,
					H1RequestBuf: firstBuf,
				}
			}
		} else {
			underlay.Write([]byte(httpLayer.Err403response))
			underlay.Close()
		}

		return
	}

	underlay = &netLayer.ReadWrapper{
		Conn:              underlay,
		OptionalReader:    io.MultiReader(firstBuf, underlay),
		RemainFirstBufLen: n,
	}

	go s.Server.ServeConn(underlay, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {
			if s.closed {
				return
			}

			//log.Println("request headers", rq.Header)

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
			} else {
				//try check customized header

				if s.Headers != nil && s.Headers.Request != nil && len(s.Headers.Request.Headers) > 0 {

					if ok, fnmk := httpLayer.AllHeadersIn(s.Headers.Request.Headers, rq.Header); !ok {

						if ce := utils.CanLogWarn("GRPC Server has custom header configured, but the client request have notMatched Header(s)"); ce != nil {
							ce.Write(zap.String("firstNotMatchKey", fnmk))
						}

						shouldFallback = true
					}

				}
			}

			if shouldFallback {
				if fallbackConnChan == nil {
					rw.WriteHeader(http.StatusNotFound)

				} else {

					if ce := utils.CanLogInfo("GrpcSimple will fallback"); ce != nil {
						ce.Write(
							zap.String("path", p),
							zap.String("method", rq.Method),
							zap.String("raddr", rq.RemoteAddr))
					}

					respConn := &netLayer.IOWrapper{
						Reader:    rq.Body,
						Writer:    rw,
						CloseChan: make(chan struct{}),
						Rejecter:  httpLayer.RejectConn{ResponseWriter: rw},
					}

					fm := httpLayer.FallbackMeta{
						Path: p,
						Conn: respConn,
					}

					fm.IsH2 = true
					fm.H2Request = rq

					if s.closed {
						return
					}

					fallbackConnChan <- fm

					<-respConn.CloseChan

				}

				return
			}

			headerMap := rw.Header()
			headerMap.Add("Content-Type", grpcContentType) //necessary

			if s.Headers != nil && s.Headers.Response != nil && len(s.Headers.Response.Headers) > 0 {
				extra := httpLayer.TrimHeaders(s.Headers.Response.Headers)
				for k, vs := range extra {
					if len(vs) > 0 {
						headerMap.Add(k, vs[0])
					}
				}
			}
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
	sc.InitEasyDeadline()

	ta, e := net.ResolveTCPAddr("tcp", rq.RemoteAddr)
	if e == nil {
		sc.ra = ta
	} else {
		if ce := utils.CanLogErr("Failed in grpcSimple parse addr; Will try X-Forwarded-For"); ce != nil {
			ce.Write(zap.String("raddr", rq.RemoteAddr), zap.Error(e))
		}

		/*
			根据 issue #106， 如果是从 nginx回落到的 grpcSimple，则这个rq.RemoteAddr 会为 "@" 导致无法转成TCPAddr,
			所以可以读一下 X-Forwarded-For

			https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/X-Forwarded-For

		*/

		xffs := rq.Header.Values(httpLayer.XForwardStr)

		if len(xffs) > 0 {
			ta, e := net.ResolveIPAddr("ip", xffs[0])
			if e == nil {
				sc.ra = ta
			} else {
				if ce := utils.CanLogErr("Failed in grpcSimple parse X-Forwarded-For"); ce != nil {
					ce.Write(zap.Error(e), zap.Any(httpLayer.XForwardStr, xffs))
				}
			}
		}
	}

	return
}

type ServerConn struct {
	commonPart

	io.Closer
	Writer http.ResponseWriter

	closeOnce sync.Once
	closeChan chan int
	closed    bool
}

// implements netLayer.RejectConn, 模仿nginx响应，参考httpLayer.Err400response_nginx
func (sc *ServerConn) Reject() {
	httpLayer.SetNginx400Response(sc.Writer)

}

func (sc *ServerConn) Close() error {
	sc.closeOnce.Do(func() {
		sc.closed = true
		close(sc.closeChan)
		if sc.Closer != nil {
			sc.Closer.Close()

		}
	})
	return nil
}

func (sc *ServerConn) Write(b []byte) (n int, err error) {

	//the determination of g.closed is necessary, or it might panic when calling Write or Flush

	if sc.closed {
		return 0, net.ErrClosed
	}

	select {
	case <-sc.WriteTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		buf := commonWrite(b)

		if sc.closed { //较为谨慎，也许commonWrite 刚调用完, 就 g.closed 了
			utils.PutBuf(buf)
			return 0, net.ErrClosed
		}
		_, err = sc.Writer.Write(buf.Bytes())
		utils.PutBuf(buf)

		if err == nil && !sc.closed {
			sc.Writer.(http.Flusher).Flush() //necessary

		}

		return len(b), err
	}

}
