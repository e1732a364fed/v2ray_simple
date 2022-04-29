package grpc

import (
	"bytes"
	"context"
	"io"
	"net"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"google.golang.org/grpc/peer"
)

// StreamConn 接口 是 stream_grpc.pb.go 中 自动生成的 Stream_TunServer 接口和 Stream_TunClient接口 的共有部分, 我们提出来.
type StreamConn interface {
	Context() context.Context
	Send(*Hunk) error
	Recv() (*Hunk, error)
}

// Conn implements net.Conn
type Conn struct {
	stream      StreamConn
	cacheReader io.Reader
	closeFunc   context.CancelFunc
	remote      net.Addr
}

func (c *Conn) Read(b []byte) (n int, err error) {
	//这里用到了一种缓存方式
	if c.cacheReader == nil {
		h, err := c.stream.Recv()
		if err != nil {
			return 0, utils.ErrInErr{ErrDesc: "grpc read failed", ErrDetail: err}
		}
		c.cacheReader = bytes.NewReader(h.Data)
	}
	n, err = c.cacheReader.Read(b)
	if err == io.EOF {
		c.cacheReader = nil
		return n, nil
	}
	return n, err
}

func (c *Conn) Write(b []byte) (n int, err error) {
	err = c.stream.Send(&Hunk{Data: b})
	if err != nil {
		return 0, utils.ErrInErr{ErrDesc: "Unable to send data over grpc stream", ErrDetail: err}
	}
	return len(b), nil
}

func (c *Conn) Close() error {
	if c.closeFunc != nil {
		c.closeFunc()
	}
	return nil
}
func (c *Conn) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   []byte{0, 0, 0, 0},
		Port: 0,
	}
}
func (c *Conn) RemoteAddr() net.Addr {
	return c.remote
}
func (*Conn) SetDeadline(time.Time) error {
	return nil
}
func (*Conn) SetReadDeadline(time.Time) error {
	return nil
}
func (*Conn) SetWriteDeadline(time.Time) error {
	return nil
}

// newConn creates Conn which handles StreamConn.
// 需要一个 cancelFunc 参数, 是因为 在 处理下一层连接的时候(如vless), 有可能出问题(如uuid不对), 并需要关闭整个 grpc连接. 我们只能通过 chan 的方式（即cancelFunc）来通知 上层进行关闭.
func newConn(service StreamConn, cancelFunc context.CancelFunc) *Conn {
	conn := &Conn{
		stream:      service,
		cacheReader: nil,
		closeFunc:   cancelFunc,
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
