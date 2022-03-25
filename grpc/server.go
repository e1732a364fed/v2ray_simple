package grpc

import (
	"context"
	"net"
	_ "unsafe"

	"google.golang.org/grpc"
)

// google.golang.org/grpc.(*Server).handleRawConn
//go:linkname HandleRawConn google.golang.org/grpc.(*Server).handleRawConn
func HandleRawConn(c *grpc.Server, lisAddr string, rawConn net.Conn)

//Server实现 grpc生成的 StreamServer 接口，用于不断处理一个客户端传来的新需求
type Server struct {
	UnimplementedStreamServer

	NewConnChan chan net.Conn

	ctx context.Context

	gs          *grpc.Server
	serviceName string
}

//  StartHandle方法 被用于 手动给 grpc提供新连接.
// 在本作中  我们不使用 grpc的listen的方法。
//非阻塞,
func (s *Server) StartHandle(conn net.Conn) {

	//非阻塞，因为 grpc.(*Server).handleRawConn 是非阻塞的，里面用了新的goroutine
	HandleRawConn(s.gs, "", conn)
}

// 该 Tun方法会被 grpc包调用, stream_TunServer就是获取到的新连接;
// 我们把该 stream_TunServer 包装成 net.Conn 并传入 NewConnChan
// 该方法是自动调用的, 我们不用管.
func (s *Server) Tun(stream_TunServer Stream_TunServer) error {
	//一般的grpc的自定义方法中,自动返回一种数据即可
	// 但是我们这里接到的是新连接, 所以这个方法就类似Accept一样;
	// 但又不全一样，因为grpc是多路复用的, 所以获取到的连接实际上来自同一条tcp连接中的一个子 “逻辑连接”

	if s.ctx == nil {
		s.ctx = context.Background()
	}

	tunCtx, cancel := context.WithCancel(s.ctx)
	s.NewConnChan <- NewConn(stream_TunServer, cancel)
	<-tunCtx.Done()

	// 这里需要一个 <-tunCtx.Done() 进行阻塞；只有当 子连接被Close的时候, Done 才能通过, 才意味着本次子连接结束.

	//正常的grpc的业务逻辑是，客户端传一大段数据上来，然后我们服务端同步传回一大段数据, 然后达到某个时间点后 就自动关闭 此次 rpc 过程调用。
	// 但是我们现在是作为代理用途， 所以到底发什么数据，和发送的时机都是在其它位置确定的,
	//  我们只能 发送一个 新连接信号，然后等待外界Close子连接.

	return nil
}

// NewServer 以 自定义 serviceName 来创建一个新 Server
// 返回一个 NewConnChan 用于接收新连接
func NewServer(serviceName string) *Server {
	gs := grpc.NewServer()

	newConnChan := make(chan net.Conn, 10)

	s := &Server{
		NewConnChan: newConnChan,
		gs:          gs,
		serviceName: serviceName,
	}

	RegisterStreamServer_withName(gs, s, serviceName)

	return s
}
