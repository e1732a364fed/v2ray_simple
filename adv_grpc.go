//go:build grpc_full

package v2ray_simple

import _ "github.com/e1732a364fed/v2ray_simple/advLayer/grpc"

//默认使用 grpcSimple，除非编译时 使用 grpc_full 这个 tag, 才会使用 grpc 包。
// go build -tags=grpc_full
