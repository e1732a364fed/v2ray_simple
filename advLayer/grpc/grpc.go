/*Package grpc implements methods for grpc.

从stream.proto 生成 stream.pb.go 和 stream_grpc.pb.go:

	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		stream.proto

stream.pb.go 可以无视, 是数据编码; 主要观察 stream_grpc.pb.go .

这里参考v2ray/xray, 它们的 Tun的意思应该是 network tunnel, 没啥的.
GunService 看不懂,可能是瞎编的; 或者可能是 "gprc tunnel"的简写吧。但是就算简写也要是 gTun。毕竟gun谐音不好。

不过查看一下v2ray的合并grpc功能的pr，
https://github.com/v2fly/v2ray-core/pull/757

似乎是来自 Qv2ray/gun 这个库；总之也一样，那么那个库的起名还是很怪。

但是因为我们使用自定义ServiceName的方法，所以似乎 GunService的名字无所谓，可以随便改

我直接把 GunService名称改成了 “Stream”，不影响的。反正我们自定义内部真实名称.

实测名称是不影响的， 一样兼容xray/v2ray的 grpc。因为 我们自定义实际的 ServiceDesc
*/
package grpc

import advlayer "github.com/hahahrfool/v2ray_simple/advLayer"

func init() {
	advlayer.ProtocolsMap["grpc"] = true
}
