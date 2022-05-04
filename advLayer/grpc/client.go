package grpc

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	clientconnMap   = make(map[netLayer.HashableAddr]ClientConn)
	clientconnMutex sync.RWMutex
)

type ClientConn *grpc.ClientConn

//获取与 某grpc服务器的 已存在的grpc连接
func GetEstablishedConnFor(addr *netLayer.Addr) ClientConn {
	clientconnMutex.RLock()
	clientconn := clientconnMap[addr.GetHashable()]
	clientconnMutex.RUnlock()

	if clientconn == nil {
		return nil
	}

	if (*grpc.ClientConn)(clientconn).GetState() != connectivity.Shutdown {
		return clientconn
	}
	//如果state是shutdown的话，我们也不用特地清除map，因为下一次申请就会覆盖map中的该项.
	//如果底层tcp被关闭，state就会为 Shutdown
	return nil
}

// ClientHandshake 在客户端被调用, 将一个普通连接升级为 grpc连接。
//该 underlay一般为 tls连接。 addr为实际的远程地址，我们不从 underlay里获取addr,避免转换.
func ClientHandshake(underlay net.Conn, addr *netLayer.Addr) (ClientConn, error) {

	// 用到了一个  clientconnMap , 可以利用现有连接,
	// 只有map里没有与对应目标远程地址的连接的时候才会拨号;
	// 这就是一种mux的实现
	// 如果有之前拨号过的client的话，直接利用现有client进行 NewStream,
	// 然后服务端的话, 实际上会获得到同一条tcp链接上的第二条子连接;

	//也就是说，底层连接在客户端-服务端只用了一条，但是 服务端处理时却会抽象出 多条Stream连接进行处理

	grpc_clientConn, err := grpc.Dial("", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, addrStr string) (net.Conn, error) {
		return underlay, nil
	}), grpc.WithConnectParams(grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  500 * time.Millisecond,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   19 * time.Second,
		},
		MinConnectTimeout: 5 * time.Second,
	}))
	if err != nil {
		return nil, err
	}

	clientconnMutex.Lock()
	clientconnMap[addr.GetHashable()] = grpc_clientConn
	clientconnMutex.Unlock()

	return grpc_clientConn, nil

}

//在一个已存在的grpc连接中 进行新的子连接申请
func DialNewSubConn(path string, clientconn ClientConn, addr *netLayer.Addr, isMulti bool) (net.Conn, error) {

	// 帮助理解：
	// 不像服务端需要自己写一个实现StreamServer接口的结构, 我们Client端直接可以调用函数生成 StreamClient
	// 这也是grpc的特点, 客户端只负责 “调用“ ”service“，而具体的service的实现 是在服务端.

	if isMulti {
		utils.Info("grpc Dialing new Sub MultiTun Conn")
	} else {
		utils.Info("grpc Dialing new Sub Conn")
	}

	streamClient := NewStreamClient((*grpc.ClientConn)(clientconn)).(streamClient_withName)

	ctx, cancelF := context.WithCancel(context.Background())
	if isMulti {
		stream_multiTunClient, err := streamClient.tunMulti_withName(ctx, path)
		if err != nil {
			clientconnMutex.Lock()
			delete(clientconnMap, addr.GetHashable())
			clientconnMutex.Unlock()
			cancelF()
			return nil, err
		}
		return newMultiConn(stream_multiTunClient, cancelF), nil
	} else {
		stream_TunClient, err := streamClient.tun_withName(ctx, path)

		if err != nil {
			clientconnMutex.Lock()
			delete(clientconnMap, addr.GetHashable())
			clientconnMutex.Unlock()
			cancelF()
			return nil, err
		}
		return newConn(stream_TunClient, cancelF), nil
	}

}

// 即 可自定义服务名的 StreamClient
type streamClient_withName interface {
	StreamClient

	tun_withName(ctx context.Context, name string, opts ...grpc.CallOption) (Stream_TunClient, error)

	tunMulti_withName(ctx context.Context, name string, opts ...grpc.CallOption) (Stream_TunMultiClient, error)
}

//比照 protoc生成的 stream_grpc.pb.go 中的 Tun方法。
// 我们加一个 tun_withName 方法, 可以自定义服务名称, 该方法让 streamClient实现 streamClient_withName 接口
func (c *streamClient) tun_withName(ctx context.Context, name string, opts ...grpc.CallOption) (Stream_TunClient, error) {
	//这里ctx不能为nil，否则会报错
	if ctx == nil {
		ctx = context.Background()
	}
	stream, err := c.cc.NewStream(ctx, &desc_withName(name).Streams[0], "/"+name+"/Tun", opts...)
	if err != nil {
		return nil, err
	}
	x := &streamTunClient{stream}
	return x, nil
}

func (c *streamClient) tunMulti_withName(ctx context.Context, name string, opts ...grpc.CallOption) (Stream_TunMultiClient, error) {
	//这里ctx不能为nil，否则会报错
	if ctx == nil {
		ctx = context.Background()
	}
	stream, err := c.cc.NewStream(ctx, &desc_withName(name).Streams[1], "/"+name+"/TunMulti", opts...)
	if err != nil {
		return nil, err
	}
	x := &streamTunMultiClient{stream}
	return x, nil
}

//implements advLayer.MuxClient
type Client struct {
	Creator
	ServerAddr netLayer.Addr
	Path       string
	ismulti    bool
}

func NewClient(addr netLayer.Addr, path string, ismulti bool) (*Client, error) {
	return &Client{
		ServerAddr: addr,
		Path:       path,
		ismulti:    ismulti,
	}, nil
}

func (c *Client) GetPath() string {
	return c.Path
}

func (c *Client) IsEarly() bool {
	return false
}

func (c *Client) GetCommonConn(underlay net.Conn) (any, error) {

	if underlay == nil {
		cc := GetEstablishedConnFor(&c.ServerAddr)
		if cc != nil {
			return cc, nil
		} else {
			return nil, utils.ErrFailed

		}
	} else {
		return ClientHandshake(underlay, &c.ServerAddr)
	}
}

func (c *Client) DialSubConn(underlay any) (net.Conn, error) {
	if underlay == nil {
		return nil, utils.ErrNilParameter
	}
	if cc, ok := underlay.(ClientConn); ok {
		return DialNewSubConn(c.Path, cc, &c.ServerAddr, c.ismulti)
	} else {
		return nil, utils.ErrWrongParameter
	}
}
