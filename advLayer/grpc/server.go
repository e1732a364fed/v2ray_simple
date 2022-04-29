package grpc

import (
	"context"
	"net"
	_ "unsafe"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"google.golang.org/grpc"
)

// 我们通过特殊方式将 grpc的私有函数映射出来, 这样方便我们使用.
//go:linkname handle_grpcRawConn google.golang.org/grpc.(*Server).handleRawConn
func handle_grpcRawConn(c *grpc.Server, lisAddr string, rawConn net.Conn)

//Server实现 grpc生成的 StreamServer 接口，用于不断处理一个客户端传来的新需求
type Server struct {
	UnimplementedStreamServer

	newConnChan chan net.Conn

	ctx context.Context

	gs          *grpc.Server
	serviceName string
}

func (s *Server) GetPath() string {
	return s.serviceName
}

func (*Server) IsMux() bool {
	return true
}

func (*Server) IsSuper() bool {
	return false
}

func (s *Server) Stop() {

}

//  StartHandle方法 被用于 手动给 grpc提供新连接.
// 在本作中  我们不使用 grpc的listen的方法。这样更加灵活.
//非阻塞. 暂不支持回落。
func (s *Server) StartHandle(conn net.Conn, theChan chan net.Conn, fallbackConnChan chan advLayer.FallbackMeta) {

	s.newConnChan = theChan

	//非阻塞，因为 grpc.(*Server).handleRawConn 是非阻塞的，里面用了新的goroutine
	handle_grpcRawConn(s.gs, "", conn)
}

// 该 Tun方法会被 grpc包调用, stream_TunServer就是获取到的新连接;
// 实际上就是在 handle_grpcRawConn 后, 每一条客户端发来的子连接 都会调用一次 s.Tun .
//
// 我们把该 stream_TunServer 包装成 net.Conn 并传入 NewConnChan
// 该方法是自动调用的, 我们不用管.
func (s *Server) Tun(stream_TunServer Stream_TunServer) error {
	//一般的grpc的自定义方法中,自动返回一种数据即可
	// 但是我们这里接到的是新连接, 所以这个方法就类似Accept一样;
	// 但又不全一样，因为grpc是多路复用的, 所以获取到的连接实际上来自同一条tcp连接中的一个子 “逻辑连接”

	utils.Info("Grpc Got New Tun")

	tunCtx, cancel := context.WithCancel(s.ctx)
	s.newConnChan <- newConn(stream_TunServer, cancel)
	<-tunCtx.Done()

	// 这里需要一个 <-tunCtx.Done() 进行阻塞；只有当 子连接被Close的时候, Done 才能通过, 才意味着本次子连接结束.

	//正常的grpc的业务逻辑是，客户端传一大段数据上来，然后我们服务端同步传回一大段数据, 然后达到某个时间点后 或者 交换信息完成后, 就 马上关闭 此次 rpc 过程调用。
	// 但是我们现在是作为代理用途， 所以到底发什么数据，和发送的时机都是在其它位置确定的,
	//  所以我们只能 发送一个 新连接信号，然后等待外界Close子连接.

	return nil
}

func (s *Server) TunMulti(stream_TunMultiServer Stream_TunMultiServer) error {

	utils.Info("Grpc Got New MultiTun")

	tunCtx, cancel := context.WithCancel(s.ctx)
	s.newConnChan <- newMultiConn(stream_TunMultiServer, cancel)
	<-tunCtx.Done()

	return nil
}

// NewServer 以 自定义 serviceName 来创建一个新 Server
func NewServer(serviceName string) *Server {
	gs := grpc.NewServer()

	s := &Server{
		gs:          gs,
		serviceName: serviceName,
		ctx:         context.Background(),
	}

	registerStreamServer_withName(gs, s, serviceName)

	return s
}

// ServerDesc_withName 用于生成指定ServiceName名称 的 grpc.ServiceDesc.
// 默认proto生成的 Stream_ServiceDesc 变量 的名称是固定的, 没办法进行自定义, 见 stream_grpc.pb.go 的最下方.
// 所以我们才使用这个办法自定义 serviceName。
func ServerDesc_withName(name string) grpc.ServiceDesc {
	return grpc.ServiceDesc{
		ServiceName: name,
		HandlerType: (*StreamServer)(nil),
		Methods:     []grpc.MethodDesc{},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "Tun",
				Handler:       _Stream_Tun_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
			{
				StreamName:    "TunMulti",
				Handler:       _Stream_TunMulti_Handler,
				ServerStreams: true,
				ClientStreams: true,
			},
		},
		Metadata: "gun.proto",
	}
}

func registerStreamServer_withName(s *grpc.Server, srv StreamServer, name string) {
	desc := ServerDesc_withName(name)
	s.RegisterService(&desc, srv)
}
