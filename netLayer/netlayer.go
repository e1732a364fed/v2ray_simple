/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, geosite, route, udp, readv, splice, relay, dns, listen/dial/sockopt 等相关功能。

以后如果要添加 kcp 或 raw socket 等底层协议时，也要在此包里实现.

*/
package netLayer

import (
	"errors"
	"io"
	"log"
	"net"
	"reflect"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	// 如果机器没有ipv6地址, 就无法联通ipv6, 此时可以在dial时更快拒绝ipv6地址,
	// 避免打印过多错误输出
	machineCanConnectToIpv6 bool

	ErrMachineCantConnectToIpv6 = errors.New("ErrMachineCanConnectToIpv6")
	ErrTimeout                  = errors.New("timeout")
)

//做一些网络层的资料准备工作, 可以优化本包其它函数的调用。
func Prepare() {
	machineCanConnectToIpv6 = HasIpv6Interface()
}

func HasIpv6Interface() bool {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		if ce := utils.CanLogErr("call net.InterfaceAddrs failed"); ce != nil {
			ce.Write(zap.Error(err))
		} else {
			log.Println("call net.InterfaceAddrs failed", err)

		}

		return false
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsPrivate() && !ipnet.IP.IsLinkLocalUnicast() {
			// IsLinkLocalUnicast: something starts with fe80:
			// According to godoc, If ip is not an IPv4 address, To4 returns nil.
			// This means it's ipv6
			if ipnet.IP.To4() == nil {

				if ce := utils.CanLogDebug("Has Ipv6Interface!"); ce != nil {
					ce.Write()
				} else {
					log.Println("Has Ipv6Interface!")
				}

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
				ce.Write(zap.Any("reader", reader), zap.String("type", reflect.TypeOf(reader).String()), zap.Error(err))
			}
			return nil
		}
		return rawConn

	}

	return nil
}

//"udp", "udp4", "udp6"
func IsStrUDP_network(s string) bool {
	switch s {
	case "udp", "udp4", "udp6":
		return true
	}
	return false
}

//选择性从 OptionalReader读取, 直到 RemainFirstBufLen 小于等于0 为止；
// 一般用于与 io.MultiReader 配合
type ReadWrapper struct {
	net.Conn
	OptionalReader    io.Reader
	RemainFirstBufLen int
}

func (mc *ReadWrapper) Read(p []byte) (n int, err error) {

	if mc.RemainFirstBufLen > 0 {
		n, err := mc.OptionalReader.Read(p)
		if n > 0 {
			mc.RemainFirstBufLen -= n
		}
		return n, err
	} else {
		return mc.Conn.Read(p)
	}

}

func (c *ReadWrapper) WriteBuffers(buffers [][]byte) (int64, error) {
	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := c.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}
