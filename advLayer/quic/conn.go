package quic

import (
	"net"
	"sync/atomic"

	"github.com/lucas-clemente/quic-go"
)

// 对 quic.Connection 的一个包装。
//用于 client 跟踪 一个 session 中 所开启的 stream的数量.
type connState struct {
	quic.Connection
	id [16]byte //这个是我们自己分配的 连接的id，不是 streamid

	openedStreamCount int32

	redialing bool
}

//implements net.Conn.
type StreamConn struct {
	quic.Stream

	// quic.Stream 与 net.Conn对比， 唯独不支持 LocalAddr 和 RemoteAddr 方法.
	// 因为它是通过 StreamID 来识别连接. 不过我们只要设置为 Connection的地址即可。
	laddr, raddr     net.Addr
	relatedConnState *connState
	isclosed         bool
}

func (sc *StreamConn) LocalAddr() net.Addr {
	return sc.laddr
}
func (sc *StreamConn) RemoteAddr() net.Addr {
	return sc.raddr
}

func (sc *StreamConn) Close() error {
	if sc.isclosed {
		return nil
	}

	sc.isclosed = true

	//这里必须要同时调用 CancelRead 和 CancelWrite
	// 因为 quic-go这个设计的是双工的，调用Close实际上只是间接调用了 CancelWrite
	// 看 quic-go包中的 quic.SendStream 的注释就知道了.

	sc.CancelRead(quic.StreamErrorCode(quic.ConnectionRefused))
	sc.CancelWrite(quic.StreamErrorCode(quic.ConnectionRefused))

	if state := sc.relatedConnState; state != nil { //服务端没有 relatedConnState

		/*
			if ce := utils.CanLogDebug("quic close"); ce != nil {
				ce.Write(
					zap.Int("count", int(rss.openedStreamCount)),
					zap.Int("ptr", int(uintptr(unsafe.Pointer(rss)))),
				)
			}
		*/

		atomic.AddInt32(&state.openedStreamCount, -1)

	}
	return sc.Stream.Close()
}
