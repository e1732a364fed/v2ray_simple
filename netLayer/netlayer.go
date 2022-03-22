/*
Package netLayer contains definitions in network layer AND transport layer.

比如路由功能一般是 netLayer去做.

以后如果要添加 domain socket, kcp 或 raw socket 等底层协议时，或者要控制tcp/udp拨号的细节时，也要在此包里实现.

*/
package netLayer

import "net"

func IsBasicConn(r interface{}) bool {
	switch r.(type) {
	case *net.TCPConn:
		return true
	case *net.UDPConn:
		return true
	case *net.UnixConn:
		return true
	}

	return false
}
