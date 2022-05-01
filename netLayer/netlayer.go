/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, geosite, route, udp, readv, splice, relay, dns, listen/dial/sockopt 等相关功能。

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
	"os"
	"reflect"
	"sync"
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
type ReadWrapper struct {
	net.Conn
	OptionalReader    io.Reader
	RemainFirstBufLen int
}

func (rw *ReadWrapper) Read(p []byte) (n int, err error) {

	if rw.RemainFirstBufLen > 0 {
		n, err := rw.OptionalReader.Read(p)
		if n > 0 {
			rw.RemainFirstBufLen -= n
		}
		return n, err
	} else {
		return rw.Conn.Read(p)
	}

}

func (rw *ReadWrapper) WriteBuffers(buffers [][]byte) (int64, error) {
	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := rw.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}

//一个自定义的由多个组件组成的实现 net.Conn 的结构
type IOWrapper struct {
	io.Reader //不可为nil
	io.Writer //不可为nil
	io.Closer
	LA, RA net.Addr

	EasyDeadline
	FirstWriteChan chan struct{} //用于确保先Write然后再Read，可为nil

	CloseChan chan struct{} //可为nil，用于接收关闭信号

	deadlineInited bool

	closeOnce      sync.Once
	firstWriteOnce sync.Once
}

func (iw *IOWrapper) Read(p []byte) (int, error) {
	if !iw.deadlineInited {
		iw.InitEasyDeadline()
		iw.deadlineInited = true
	}
	select {
	case <-iw.ReadTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		if iw.FirstWriteChan != nil {
			<-iw.FirstWriteChan
			return iw.Reader.Read(p)
		} else {
			return iw.Reader.Read(p)

		}
	}
}

func (iw *IOWrapper) Write(p []byte) (int, error) {

	if iw.FirstWriteChan != nil {
		defer iw.firstWriteOnce.Do(func() {
			close(iw.FirstWriteChan)
		})

	}

	if !iw.deadlineInited {
		iw.InitEasyDeadline()
		iw.deadlineInited = true
	}
	select {
	case <-iw.WriteTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		return iw.Writer.Write(p)
	}
}

func (iw *IOWrapper) Close() error {
	if c := iw.Closer; c != nil {
		return c.Close()
	}
	if iw.CloseChan != nil {
		iw.closeOnce.Do(func() {
			close(iw.CloseChan)
		})

	}
	return nil
}
func (iw *IOWrapper) LocalAddr() net.Addr  { return iw.LA }
func (iw *IOWrapper) RemoteAddr() net.Addr { return iw.RA }
