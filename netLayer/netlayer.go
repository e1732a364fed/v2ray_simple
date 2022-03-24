/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, readv copy, route, udp 等相关功能。

以后如果要添加 domain socket, kcp 或 raw socket 等底层协议时，或者要控制tcp/udp拨号的细节时，也要在此包里实现.

*/
package netLayer

import (
	"io"
	"log"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
)

//net.IPConn, net.TCPConn, net.UDPConn, net.UnixConn
func IsBasicConn(r interface{}) bool {
	if _, ok := r.(syscall.Conn); ok {
		return true
	}

	return false
}

func GetRawConn(reader io.Reader) syscall.RawConn {
	if sc, ok := reader.(syscall.Conn); ok {
		rawConn, err := sc.SyscallConn()
		if err != nil {
			if utils.CanLogDebug() {
				log.Println("can't convert syscall.Conn to syscall.RawConn", reader, err)
			}
			return nil
		}
		return rawConn

	}

	return nil
}
