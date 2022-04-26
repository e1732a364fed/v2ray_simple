package quic

import (
	"net"
	"sync/atomic"

	"github.com/lucas-clemente/quic-go"
)

// 对 quic.Connection 的一个包装。
//用于 跟踪 一个 session 中 所开启的 stream的数量.
type connState struct {
	quic.Connection
	id [16]byte

	openedStreamCount int32
}

//给 quic.Stream 添加 方法使其满足 net.Conn.
// quic.Stream 唯独不支持 LocalAddr 和 RemoteAddr 方法.
// 因为它是通过 StreamID 来识别连接. 不过session是有的。
type StreamConn struct {
	quic.Stream
	laddr, raddr     net.Addr
	relatedConnState *connState
	isclosed         bool
}

func (sc StreamConn) LocalAddr() net.Addr {
	return sc.laddr
}
func (sc StreamConn) RemoteAddr() net.Addr {
	return sc.raddr
}

//这里必须要同时调用 CancelRead 和 CancelWrite
// 因为 quic-go这个设计的是双工的，调用Close实际上只是间接调用了 CancelWrite
// 看 quic-go包中的 quic.SendStream 的注释就知道了.
func (sc StreamConn) Close() error {
	if sc.isclosed {
		return nil
	}
	sc.isclosed = true
	sc.CancelRead(quic.StreamErrorCode(quic.ConnectionRefused))
	sc.CancelWrite(quic.StreamErrorCode(quic.ConnectionRefused))
	if rss := sc.relatedConnState; rss != nil {

		atomic.AddInt32(&rss.openedStreamCount, -1)

	}
	return sc.Stream.Close()
}
