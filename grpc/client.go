package grpc

import (
	"context"
	"net"
	"net/netip"
	sync "sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	globalDialerMap    map[netip.AddrPort]*grpc.ClientConn
	globalDialerAccess sync.Mutex
)

func ClientHandshake(underlay net.Conn) (net.Conn, error) {

	//v2ray的实现中用到了一个  globalDialerMap[dest], 可以利用现有连接,
	// 只有map里没有与对应目标远程地址的连接的时候才会拨号;
	// 这 应该就是一种mux的实现
	// 如果有之前播过的client的话，直接利用现有client进行 NewStream, 然后服务端的话, 实际上会获得到第二条连接;

	//也就是说，底层连接在客户端-客户端只用了一条，但是 处理时却会抽象出 多条Stream连接进行处理

	grpc_clientConn, err := grpc.Dial("", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, addrStr string) (net.Conn, error) {
		return underlay, nil
	}))
	if err != nil {
		return nil, err
	}

	//不像服务端需要自己写一个实现StreamServer接口的结构, 我们Client端直接可以调用函数生成 StreamClient
	// 这也是grpc的特点, 客户端只负责 “调用“ ”service“，而具体的service的实现 是在服务端.

	streamClient := NewStreamClient(grpc_clientConn).(StreamClient_withName)

	stream_TunClient, err := streamClient.Tun_withName(nil, "mypath1")
	if err != nil {
		return nil, err
	}
	return NewConn(stream_TunClient, nil), nil

}
