/*Package grpc implements methods for grpc tunnel.

ProtoBuf

从stream.proto 生成 stream.pb.go 和 stream_grpc.pb.go:

	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		stream.proto

stream.pb.go 可以无视, 是数据编码; 主要观察 stream_grpc.pb.go .

Naming of gun

这里参考v2ray/xray, 它们的 Tun的意思应该是 network tunnel, 没啥的.
GunService 看不懂, 可能是 "gprc tunnel"的简写吧。但是就算简写也要是 gTun。毕竟gun谐音不好。

查看一下v2ray的合并grpc功能的pr，
https://github.com/v2fly/v2ray-core/pull/757

是来自 Qv2ray/gun 这个库；总之也一样，那个库的起名还是很怪。

因为我们使用自定义ServiceName的方法，所以 GunService的名字无所谓，可以随便改.

直接把 GunService名称改成了 “Stream”，不影响的。反正我们自定义内部真实名称.

实测 是不影响的， 一样兼容xray/v2ray的 grpc, 因为 我们自定义实际的 ServiceDesc.

Client Methods

建立新客户端连接的调用过程：

先用 GetEstablishedConnFor 看看有没有已存在的 clientConn

没有 已存在的 时，自己先拨号tcp，然后拨号tls，然后把tls连接 传递给 ClientHandshake, 生成一个 clientConn

然后把获取到的 clientConn传递给 DialNewSubConn, 获取可用的一条 grpc 连接


*/
package grpc

import (
	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	advLayer.ProtocolsMap["grpc"] = Creator{}
}

type Creator struct{}

func (Creator) PackageID() string {
	return "grpc"
}

func (Creator) ProtocolName() string {
	return "grpc"
}

func (Creator) CanHandleHeaders() bool {
	return false
}

func (Creator) IsSuper() bool {
	return false
}

func (Creator) IsMux() bool {
	return true
}

func (Creator) GetDefaultAlpn() (alpn string, mustUse bool) {
	// v2ray 和 xray 的grpc 因为没有自己处理tls，直接用grpc包处理的tls，而grpc包对alpn有严格要求, 要用h2.
	return "h2", true
}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {
	grpc_multi := false
	if extra := conf.Extra; len(extra) > 0 {
		if thing := extra["grpc_multi"]; thing != nil {

			if v, ok := utils.AnyToBool(thing); ok {
				grpc_multi = v
			}
		}
	}

	return NewClient(conf.Addr, conf.Path, grpc_multi)
}

func (Creator) NewServerFromConf(conf *advLayer.Conf) (advLayer.Server, error) {
	return NewServer(conf.Path), nil
}
