package grpcSimple

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"sync"

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
func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn) {
	go s.Server.ServeConn(underlay, &http2.ServeConnOpts{
		Handler: http.HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) {

			//log.Println("request headers", rq.Header)

			//TODO: support fallback

			if rq.URL.Path != s.path {
				if ce := utils.CanLogWarn("grpc Server got wrong path"); ce != nil {
					ce.Write(zap.String("path", rq.URL.Path))
				}

				return
			}

			if ct := rq.Header.Get("Content-Type"); ct != "application/grpc" {
				if ce := utils.CanLogWarn("GRPC Server got wrong Content-Type"); ce != nil {
					ce.Write(zap.String("type", ct))
				}

				return
			}

			//https://dzone.com/articles/learning-about-the-headers-used-for-grpc-over-http

			headerMap := rw.Header()
			headerMap.Add("Content-Type", "application/grpc") //necessary
			rw.WriteHeader(http.StatusOK)

			cc := make(chan int)
			sc := &ServerConn{
				br:        bufio.NewReader(rq.Body),
				Writer:    rw,
				Closer:    rq.Body,
				closeChan: cc,
			}

			sc.timeouter = timeouter{
				closeFunc: func() {
					sc.Close()
				},
			}
			newSubConnChan <- sc
			<-cc //necessary
		}),
	})
}

type ServerConn struct {
	io.Closer
	io.Writer

	remain int
	br     *bufio.Reader

	once      sync.Once
	closeChan chan int

	timeouter
}

func (g *ServerConn) Close() error {
	g.once.Do(func() {
		close(g.closeChan)
		g.Closer.Close()
	})
	return nil
}

func (g *ServerConn) Read(b []byte) (n int, err error) {

	if g.remain > 0 {

		size := g.remain
		if len(b) < size {
			size = len(b)
		}

		n, err = io.ReadFull(g.br, b[:size])
		g.remain -= n
		return
	}

	_, err = g.br.Discard(6)
	if err != nil {

		return 0, err
	}

	protobufPayloadLen, err := binary.ReadUvarint(g.br)
	if err != nil {
		return 0, ErrInvalidLength
	}

	size := int(protobufPayloadLen)
	if len(b) < size {
		size = len(b)
	}

	n, err = io.ReadFull(g.br, b[:size])
	if err != nil {
		return
	}

	remain := int(protobufPayloadLen) - n
	if remain > 0 {
		g.remain = remain
	}
	return n, nil
}

func (g *ServerConn) Write(b []byte) (n int, err error) {

	protobufHeader := [binary.MaxVarintLen64 + 1]byte{0x0A}
	varuintSize := binary.PutUvarint(protobufHeader[1:], uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(varuintSize + 1 + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)

	buf := utils.GetBuf()
	defer utils.PutBuf(buf)
	buf.Write(grpcHeader)
	buf.Write(protobufHeader[:varuintSize+1])
	buf.Write(b)

	_, err = g.Writer.Write(buf.Bytes())

	if err == nil {
		g.Writer.(http.Flusher).Flush() //necessary

	}

	return len(b), err
}
