package grpc

import (
	"bytes"
	"context"
	"io"
	"net"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
	"google.golang.org/grpc/peer"
)

type StreamMultiConn interface {
	Context() context.Context
	Send(*MultiHunk) error
	Recv() (*MultiHunk, error)
}

//实现 net.Conn 和 utils.MultiReader
type MultiConn struct {
	stream      StreamMultiConn
	cacheReader io.Reader
	closeFunc   context.CancelFunc
	local       net.Addr
	remote      net.Addr
}

func (c *MultiConn) Read(b []byte) (n int, err error) {
	//这里用到了一种缓存方式
	if c.cacheReader == nil {
		var h *MultiHunk
		h, err = c.stream.Recv()
		if err != nil {
			return 0, utils.ErrInErr{ErrDesc: "grpc read MultiConn failed", ErrDetail: err}
		}
		switch len(h.Data) {
		case 0:
			return
		case 1:
			c.cacheReader = bytes.NewReader(h.Data[0])

		default:
			result, _ := utils.MergeBuffers(h.Data)
			c.cacheReader = bytes.NewReader(result)
		}

	}
	n, err = c.cacheReader.Read(b)
	if err == io.EOF {
		c.cacheReader = nil
		return n, nil
	}
	return n, err
}

func (c *MultiConn) ReadBuffers() (bs [][]byte, err error) {

	h, errx := c.stream.Recv()

	if errx != nil {
		err = utils.ErrInErr{ErrDesc: "grpc ReadBuffers MultiConn failed", ErrDetail: errx}
		return
	}
	bs = h.Data
	return

}

func (c *MultiConn) WillReadBuffersBenifit() bool {
	return true
}

func (c *MultiConn) Write(b []byte) (n int, err error) {
	err = c.stream.Send(&MultiHunk{Data: [][]byte{b}})
	if err != nil {
		return 0, utils.ErrInErr{ErrDesc: "Unable to send data over grpc stream MultiConn", ErrDetail: err}
	}
	return len(b), nil
}

func (c *MultiConn) WriteBuffers(bs [][]byte) (num int64, err error) {

	err = c.stream.Send(&MultiHunk{Data: bs})
	if err != nil {
		return 0, utils.ErrInErr{ErrDesc: "Unable to send data over grpc stream MultiConn", ErrDetail: err}
	}
	num = int64(utils.BuffersLen(bs))
	return
}

func (c *MultiConn) Close() error {
	if c.closeFunc != nil {
		c.closeFunc()
	}
	return nil
}
func (c *MultiConn) LocalAddr() net.Addr {
	return c.local
}
func (c *MultiConn) RemoteAddr() net.Addr {
	return c.remote
}
func (*MultiConn) SetDeadline(time.Time) error {
	return nil
}
func (*MultiConn) SetReadDeadline(time.Time) error {
	return nil
}
func (*MultiConn) SetWriteDeadline(time.Time) error {
	return nil
}

func newMultiConn(service StreamMultiConn, cancelFunc context.CancelFunc) *MultiConn {
	conn := &MultiConn{
		stream:      service,
		cacheReader: nil,
		closeFunc:   cancelFunc,
	}

	conn.local = &net.TCPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	}
	ad, ok := peer.FromContext(service.Context())
	if ok {
		conn.remote = ad.Addr
	} else {
		conn.remote = &net.TCPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: 0,
		}
	}

	return conn
}
