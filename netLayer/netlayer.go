/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, geosite, route, udp, readv, splice, relay, dns, listen/dial/sockopt, proxy protocol 等相关功能。

以后如果要添加 kcp 或 raw socket 等底层协议时，也要在此包 或子包里实现.

Tags

本包提供 embed_geoip 这个 build tag。

若给出 embed_geoip，则会尝试内嵌 GeoLite2-Country.mmdb.tgz 文件；默认不内嵌。

*/
package netLayer

import (
	"errors"
	"io"
	"log"
	"net"
	"reflect"
	"syscall"
	"time"

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

//c.SetDeadline(time.Time{})
func PersistConn(c net.Conn) {
	c.SetDeadline(time.Time{})
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

// part of net.Conn
type NetAddresser interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// part of net.Conn
type NetDeadliner interface {
	SetDeadline(t time.Time) error

	// SetReadDeadline sets the deadline for future Read calls
	// and any currently-blocked Read call.
	// A zero value for t means Read will not time out.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for future Write calls
	// and any currently-blocked Write call.
	// Even if write times out, it may return n > 0, indicating that
	// some of the data was successfully written.
	// A zero value for t means Write will not time out.
	SetWriteDeadline(t time.Time) error
}

//实现 NetAddresser
type EasyNetAddresser struct {
	LA, RA net.Addr
}

func (iw *EasyNetAddresser) LocalAddr() net.Addr  { return iw.LA }
func (iw *EasyNetAddresser) RemoteAddr() net.Addr { return iw.RA }

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
