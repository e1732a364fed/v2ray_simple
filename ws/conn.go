package ws

import (
	"io"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/utils"
)

// 因为 gobwas/ws 不包装conn，在写入和读取二进制时需要使用 较为底层的函数才行，并未被提供标准的Read和Write
// 因此我们包装一下，统一使用Read和Write函数 来读写 二进制数据。因为我们这里是代理，
// 所以我们默认 抛弃 websocket的 数据帧 长度。
// 如果以后考虑与 vless v1的 udp 相结合的话，则数据包长度 不能丢弃，需要使用另外的实现。
type Conn struct {
	net.Conn
	first_nextFrameCalled bool

	state ws.State
	r     *wsutil.Reader
	//w     *wsutil.Writer //wsutil.Writer会在内部进行缓存,并且有时会分片发送,降低性能,不建议使用

	remainLenForLastFrame int64

	serverEndGotEarlyData []byte
}

//Read websocket binary frames
func (c *Conn) Read(p []byte) (int, error) {

	//log.Println("real ws read called", len(p))

	if len(c.serverEndGotEarlyData) > 0 {
		n := copy(p, c.serverEndGotEarlyData)
		c.serverEndGotEarlyData = c.serverEndGotEarlyData[n:]
		return n, nil
	}

	//websocket 协议中帧长度上限为2^64，超大，考虑到我们vs的标准Packet缓存是64k，也就是2^16,
	// https://www.rfc-editor.org/rfc/rfc6455#section-5.2
	// (使用了 Extended payload length 字段)
	// 肯定会有多读的情况，此时如果一次用 wsutil.ReadServerBinary()的话，那么服务器缓存就超大，不可能如此实现
	// （wsutil.ReadServerBinary内部使用了 io.ReadAll, 而ReadAll是会无限增长内存的
	// 所以我们肯定要分段读， 直接用 wsutil.Reader.Read 即可， 但是每个Read前必须要有 NextFrame调用
	//
	//关于读 的完整过程，建议参考 ws/example.autoban.go 里的 wsHandler 函数

	if c.remainLenForLastFrame > 0 {
		//log.Println("c.remainLenForLastFrame > 0", c.remainLenForLastFrame)
		n, e := c.r.Read(p)
		//log.Println("c.remainLenForLastFrame > 0, read ok", n, e, c.remainLenForLastFrame-int64(n))
		if e != nil && e != io.EOF {
			return n, e
		}
		c.remainLenForLastFrame -= int64(n)
		// 这里之所以可以放心 减去 n，不怕减成负的，是因为 r的代码里 在读取一帧的数据时，用到了 io.LimitedReader, 一帧的读取长度的上限已被限定，直到 该帧完全被读完为止
		return n, nil
	}

	h, e := c.r.NextFrame()
	if e != nil {
		return 0, e
	}
	if h.OpCode.IsControl() {
		//log.Println("Got control frame")
		//return 0, nil

		// 控制帧已经在我们的 OnIntermediate 里被处理了, 直接读取下一个数据即可
		return c.Read(p)
	}

	// 发现读取分片数据时，会遇到 OpContinuation, 之前直接当错误了所以导致读取失败
	if h.OpCode != ws.OpBinary && h.OpCode != ws.OpContinuation {

		/*一共只有这几种，剩下的 Ping,Pong 和 Text都不是我们想要的
		OpContinuation OpCode = 0x0
		OpText         OpCode = 0x1
		OpBinary       OpCode = 0x2
		OpClose        OpCode = 0x8
		OpPing         OpCode = 0x9
		OpPong         OpCode = 0xa
		*/

		//log.Println("OpCode not Binary", h.OpCode)
		return 0, utils.NewDataErr("ws OpCode not Binary", nil, h.OpCode)
	}
	//log.Println("Read next frame header ok,", h.Length, c.r.State.Fragmented(), "givenbuf len", len(p))

	c.remainLenForLastFrame = h.Length

	//r.NextFrame 内部会使用一个 io.LimitedReader, 每次Read到的最大长度为一个Frame首部所标明的大小
	// io.LimitedReader 最后一次Read，是会返回EOF的， 但是这种正常的EOF已经被 wsutil.Reader.Read处理了

	// 见 https://github.com/gobwas/ws/blob/2effe5ec7e4fd737085eb56beded3ad6491269dc/wsutil/reader.go#L125

	// 但是后来发现，只有 fragmented的情况下，才会处理EOF，否则还是会传递到我们这里
	// 也就是说，websocket虽然一个数据帧可以超大，但是 还有一种 分片功能，而如果不分片的话，gobwas就不处理EOF
	//经过实测，如果数据比较小的话，就不会分片，此时就会出现EOF; 如果数据比较大，比如 4327，就要分片

	//这种产生EOF的情况，时 gobwas/ws包的一种特性，这样可以说每一次读取都能有明确的EOF边界，便于使用 io.ReadAll

	n, e := c.r.Read(p)
	//log.Println("read data result", e, n, h.Length)

	c.remainLenForLastFrame -= int64(n)

	if e != nil && e != io.EOF {

		//log.Println("e", e, n, string(p[:n]), p[:n])
		return n, e

	}
	return n, nil
}

//Write websocket binary frames
func (c *Conn) Write(p []byte) (n int, e error) {
	//log.Println("ws Write called", len(p))

	//查看了代码，wsutil.WriteClientBinary 等类似函数会直接调用 ws.WriteFrame， 是不分片的.
	// 不分片的效率更高,因为无需缓存,zero copy

	if c.state == ws.StateClientSide {
		e = wsutil.WriteClientBinary(c.Conn, p)
	} else {
		e = wsutil.WriteServerBinary(c.Conn, p)
	}
	//log.Println("ws Write finished", n, e, len(p))

	if e == nil {
		n = len(p)
	} else {
		//log.Println(e)
	}
	return

	//仔细查看这个Write代码，它在DisableFlush没有被调用过时，
	//每次都将p写入自己的缓存，而不flush; 然后等待用户自己调用Flush

	//如果一次p写入的过长，那么它就不等待用户调用Flush 而自己先调用 w.flushFragment(false),
	// 这个false的意思就是，不在websocket数据头里添加Fin标志

	//如果调用过了 DisableFlush，则 w不会去使用 flushFragment 进行分片发送; 而是去试图增长自己的buffer
	// 总之实际上这个 wsutil的这种带buffer的情况并不适合我们的 代理服务的高效转发,我们就直接写入底层连接就行,
	// 不必使用缓存

	//另外，“分片”的判断，就是，只要头部没有Fin标志，那就是分片的.

	//在这里将我的分析过程写在这里，方便同学们学习，避免重蹈覆辙

	// 同时也留着这段代码，方便测试 分片 Write 时使用

	/*
		n, e = c.w.Write(p)
		if e == nil {
			//发现必须要调用Flush，否则根本不会写入.
			// 但是因为我们一般 vless 的头部是直接附带数据的，所以我们要是第一次就Flush的话可能有点浪费。

			// 在调用了 DisableFlush 方法后，还是必须要调用 Flush, 否则还是什么也不写入
			e = c.w.Flush()

			//似乎Flush之后还要Reset？不知道是不是没Reset 导致了 分片时 读取出问题的情况
		}
		//log.Println("ws Write finish", n, e)
		return
	*/

}
