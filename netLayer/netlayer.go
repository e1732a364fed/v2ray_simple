/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, readv, relay, route, udp, splice 等相关功能。

以后如果要添加 kcp 或 raw socket 等底层协议时，或者要控制tcp/udp拨号的细节时，也要在此包里实现.

*/
package netLayer

import (
	"io"
	"log"
	"net"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	// 如果机器没有ipv6地址, 就无法联通ipv6, 此时可以在dial时更快拒绝ipv6地址,
	// 避免打印过多错误输出
	machineCanConnectToIpv6 bool

	ErrMachineCanConnectToIpv6 = utils.NumErr{Prefix: "ErrMachineCanConnectToIpv6"}
)

//做一些网络层的资料准备工作, 可以优化本包其它函数的调用。
func Prepare() {
	machineCanConnectToIpv6 = HasIpv6Interface()
}

func HasIpv6Interface() bool {

	if utils.LogLevel == utils.Log_debug {
		log.Println("HasIpv6Interface called")
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		if utils.ZapLogger != nil {
			if ce := utils.CanLogErr("call net.InterfaceAddrs failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		} else {
			log.Println("call net.InterfaceAddrs failed", err)
		}

		return false
	}

	if utils.LogLevel == utils.Log_debug {

		log.Println("interfaces", len(addrs), addrs)

		for _, address := range addrs {

			if ipnet, ok := address.(*net.IPNet); ok {

				isipv6 := false

				if !ipnet.IP.IsLoopback() && !ipnet.IP.IsPrivate() && !ipnet.IP.IsLinkLocalUnicast() {
					if ipnet.IP.To4() == nil {
						isipv6 = true
					}
				}
				log.Println(ipnet.IP.String(), isipv6)

			}

		}
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsPrivate() && !ipnet.IP.IsLinkLocalUnicast() {
			// IsLinkLocalUnicast: something starts with fe80:
			// According to godoc, If ip is not an IPv4 address, To4 returns nil.
			// This means it's ipv6
			if ipnet.IP.To4() == nil {
				return true
			}
		}
	}
	return false
}

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
			if ce := utils.CanLogDebug("can't convert syscall.Conn to syscall.RawConn"); ce != nil {
				ce.Write(zap.Any("reader", reader), zap.Error(err))
			}
			return nil
		}
		return rawConn

	}

	return nil
}

func IsStrUDP_network(s string) bool {
	switch s {
	case "udp", "udp4", "udp6":
		return true
	}
	return false
}
