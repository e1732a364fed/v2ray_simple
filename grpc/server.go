package grpc

import (
	"context"
	"net"
	_ "unsafe"

	"google.golang.org/grpc"
)

//go:linkname HandleRawConn google.golang.org/grpc.(*Server).handleRawConn
func HandleRawConn(c *grpc.Server, lisAddr string, rawConn net.Conn)

//Server实现 grpc生成的 StreamServer 接口
type Server struct {
	UnimplementedStreamServer

	NewConnChan chan net.Conn

	ctx context.Context
}

// 该 Tun方法会被 grpc包调用, stream_TunServer就是获取到的新连接;
// 我们把该 stream_TunServer 包装成 net.Conn 并传入 NewConnChan
func (s *Server) Tun(stream_TunServer Stream_TunServer) error {
	//一般的grpc的自定义方法中,自动返回一种数据即可
	// 但是我们这里接到的是新连接, 所以这个方法就类似Accept一样;
	// 但又不全一样，因为grpc是多路复用的, 所以获取到的连接实际上来自同一条tcp连接中的一个子 “逻辑连接”

	tunCtx, cancel := context.WithCancel(s.ctx)
	s.NewConnChan <- NewConn(stream_TunServer, cancel)
	<-tunCtx.Done()

	return nil
}

// NewServer 以 自定义 serviceName 来创建一个新 Server
// 返回一个 NewConnChan 用于接收新连接
func NewServer(serviceName string) chan net.Conn {
	gs := grpc.NewServer()

	newConnChan := make(chan net.Conn, 10)

	s := &Server{
		NewConnChan: newConnChan,
	}

	RegisterStreamServer_withName(gs, s, serviceName)

	return newConnChan
}
