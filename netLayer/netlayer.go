/*
Package netLayer contains definitions in network layer AND transport layer.

本包有 geoip, geosite, route, udp, readv, splice, relay, dns, listen/dial/sockopt, proxy protocol 等相关功能。

以后如果要添加 kcp 或 raw socket 等底层协议时，也要在此包 或子包里实现.

# Tags

本包提供 embed_geoip 这个 build tag。

若给出 embed_geoip，则会尝试内嵌 GeoLite2-Country.mmdb.tgz 文件；默认不内嵌。
*/
package netLayer

import (
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"syscall"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	ErrTimeout = errors.New("timeout")
)

// c.SetDeadline(time.Time{})
func PersistConn(c net.Conn) {
	c.SetDeadline(time.Time{})
}

func IsTCP(r any) *net.TCPConn {
	if tc, ok := r.(*net.TCPConn); ok {
		return tc
	}

	return nil
}

// net.IPConn, net.TCPConn, net.UDPConn, net.UnixConn
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

// "udp", "udp4", "udp6"
func IsStrUDP_network(s string) bool {
	switch s {
	case "udp", "udp4", "udp6":
		return true
	}
	return false
}

// 返回它所包装前的 那一层 net.Conn, 不一定是 基本连接，
// 所以仍然可以继续 被识别为 ConnWrapper 并继续解包.
type ConnWrapper interface {
	Upstream() net.Conn
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

// 实现 NetAddresser
type EasyNetAddresser struct {
	LA, RA net.Addr
}

func (iw *EasyNetAddresser) LocalAddr() net.Addr  { return iw.LA }
func (iw *EasyNetAddresser) RemoteAddr() net.Addr { return iw.RA }

// 用于定义拒绝响应的行为；可参考 httpLayer.RejectConn
type RejectConn interface {
	RejectBehaviorDefined() bool //若为false，则只能直接Close
	Reject()
}

type TCPRequestInfo struct {
	net.Conn
	Target Addr
}

type UDPRequestInfo struct {
	MsgConn
	Target Addr
}

type ChanCloseConn struct {
	net.Conn
	CChan chan struct{}
}

func (c *ChanCloseConn) Close() error {
	close(c.CChan)
	return nil
}

// ConnList 是一个 多线程安全 的用于保存Conn的列表
type ConnList struct {
	sync.Mutex
	list []net.Conn
}

func (cl *ConnList) Insert(c net.Conn) {
	cl.Lock()
	cl.list = append(cl.list, c)
	cl.Unlock()
}

func (cl *ConnList) Remove(c net.Conn) {
	cl.Lock()

	index := -1
	for i, v := range cl.list {
		if v == c {
			index = i
		}
	}
	if index == -1 {
		cl.Unlock()
		return
	}
	utils.Splice(&cl.list, index, 1)
	cl.Unlock()
}

func (cl *ConnList) CloseAndRemoveAll() {
	cl.Lock()
	for _, conn := range cl.list {
		conn.Close()
	}
	cl.list = nil
	cl.Unlock()

}
