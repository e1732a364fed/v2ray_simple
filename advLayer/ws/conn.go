package ws

import (
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// 实现 net.Conn, io.ReaderFrom, utils.MultiWriter, netLayer.Splicer
// 因为 gobwas/ws 不包装conn，在写入和读取二进制时需要使用 较为底层的函数才行，并未被提供标准的Read和Write。
// 因此我们包装一下，统一使用Read和Write函数 来读写 二进制数据。因为我们这里是代理，
type Conn struct {
	net.Conn

	state ws.State
	r     *wsutil.Reader
	//w     *wsutil.Writer //wsutil.Writer会在内部进行缓存,并且有时会分片发送,降低性能,不建议使用

	remainLenForLastFrame int64

	serverEndGotEarlyData []byte

	underlayIsTCP   bool
	underlayIsBasic bool

	bigHeaderEverUsed bool

	realRaddr net.Addr //可从 X-Forwarded-For 读取用户真实ip，用于反代等情况
}

// Read websocket binary frames
func (c *Conn) Read(p []byte) (int, error) {

	if len(c.serverEndGotEarlyData) > 0 {
		n := copy(p, c.serverEndGotEarlyData)
		c.serverEndGotEarlyData = c.serverEndGotEarlyData[n:]
		return n, nil
	}

	//websocket 协议中帧长度上限为2^64，超大，考虑到我们vs的标准Packet缓存是64k，也就是2^16,
	// https://www.rfc-editor.org/rfc/rfc6455#section-5.2
	// (使用了 Extended payload length 字段)
	// 肯定会有多读的情况，此时如果一次用 wsutil.ReadServerBinary()的话，那么服务器缓存就超大，不可能如此实现
	// ( wsutil.ReadServerBinary内部使用了 io.ReadAll, 而ReadAll是会无限增长内存的 )
	// 所以我们肯定要分段读， 直接用 wsutil.Reader.Read 即可， 注意 每个Read前必须要有 NextFrame调用
	//
	//关于读 的完整过程，建议参考 ws/example.autoban.go 里的 wsHandler 函数

	if c.remainLenForLastFrame > 0 {

		n, e := c.r.Read(p)

		if e != nil && e != io.EOF {
			return n, e
		}
		c.remainLenForLastFrame -= int64(n)
		// 这里之所以可以放心 减去 n，不怕减成负的，是因为 wsutil.Reader 的代码里 在读取一帧的数据时，用到了 io.LimitedReader, 一帧的读取长度的上限已被限定，直到 该帧完全被读完为止
		return n, nil
	}

	h, e := c.r.NextFrame()
	if e != nil {
		return 0, e
	}
	if h.OpCode.IsControl() {

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

		return 0, utils.ErrInErr{ErrDesc: "ws OpCode not OpBinary/OpContinuation", Data: h.OpCode}
	}

	c.remainLenForLastFrame = h.Length

	//r.NextFrame 内部会使用一个 io.LimitedReader, 每次Read到的最大长度为一个Frame首部所标明的大小
	// io.LimitedReader 最后一次Read，是会返回EOF的， 但是这种正常的EOF已经被 wsutil.Reader.Read处理了

	// 见 https://github.com/gobwas/ws/blob/2effe5ec7e4fd737085eb56beded3ad6491269dc/wsutil/reader.go#L125

	// 但是后来发现，只有 fragmented的情况下，才会处理EOF，否则还是会传递到我们这里
	// 也就是说，websocket虽然一个数据帧可以超大，但是 还有一种 分片功能，而如果不分片的话，gobwas就不处理EOF
	//经过实测，如果数据比较小的话，就不会分片，此时就会出现EOF; 如果数据比较大，比如 4327，某客户端就可能选择分片

	//这种产生EOF的情况，是 gobwas/ws包的一种特性，这样可以说每一次读取都能有明确的EOF边界，便于使用 io.ReadAll

	n, e := c.r.Read(p)

	c.remainLenForLastFrame -= int64(n)

	if e != nil && e != io.EOF {

		return n, e

	}
	return n, nil
}

func (c *Conn) EverPossibleToSpliceWrite() bool {
	return c.underlayIsTCP && c.state == ws.StateServerSide
}

// 采用 “超长包” 的办法 试图进行splice
func (c *Conn) tryWriteBigHeader() (e error) {
	if c.bigHeaderEverUsed {
		return

	}

	c.bigHeaderEverUsed = true
	wsH := ws.Header{
		Fin:    true,
		OpCode: ws.OpBinary,
		Length: 1 << 62,
	}

	e = ws.WriteHeader(c.Conn, wsH)
	return
}

func (c *Conn) CanSpliceWrite() (r bool, conn *net.TCPConn) {
	if !c.EverPossibleToSpliceWrite() {
		return
	}

	if c.tryWriteBigHeader() != nil {
		return
	}

	if tc := netLayer.IsTCP(c.Conn); tc != nil {
		r = true
		conn = tc

	}

	return

}

func (c *Conn) ReadFrom(r io.Reader) (written int64, err error) {
	if c.state == ws.StateClientSide {
		return utils.ClassicCopy(c, r)
	}

	//采用 “超长包” 的办法 试图进行splice

	e := c.tryWriteBigHeader()
	if e != nil {
		return 0, e
	}
	if c.underlayIsTCP {
		if rt, ok := c.Conn.(io.ReaderFrom); ok {
			return rt.ReadFrom(r)
		} else {
			panic("ws.Conn underlayIsTCP, but can't cast to ReadFrom")
		}
	}

	return netLayer.TryReadFrom_withSplice(c, c.Conn, r, func() bool {
		return true
	})
}

// 实现 utils.MultiWriter
// 主要是针对一串数据的情况，如果底层连接可以用writev， 此时我们不要每一小段都包包头 然后写N次，
// 而是只在最前面包数据头，然后即可用writev 一次发送出去
// 比如从 socks5 读数据，写入 tcp +ws + vless 协议, 就是这种情况
// 若底层是tls，那我们也合并再发出，这样能少些很多头部,也能减少Write次数
func (c *Conn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.underlayIsBasic {
		allLen := utils.BuffersLen(buffers)

		if c.state == ws.StateClientSide {

			//如果是客户端，需要将全部数据进行掩码处理，超烦人的！
			//我们直接将所有数据合并到一起, 然后自行写入 frame, 而不是使用 wsutil的函数，能省内存拷贝开销

			bigbs, dup := utils.MergeBuffers(buffers)
			frame := ws.NewFrame(ws.OpBinary, true, bigbs)
			frame = ws.MaskFrameInPlace(frame)
			e := ws.WriteFrame(c.Conn, frame)
			if dup {
				utils.PutPacket(bigbs)
			}

			if e != nil {
				return 0, e
			}
			return int64(allLen), nil
		} else {
			//如果是服务端，因为无需任何对数据的修改，我们就可以连续将分片的数据依次直接写入,达到加速效果

			wsH := ws.Header{
				Fin:    true,
				OpCode: ws.OpBinary,
				Length: int64(allLen),
			}

			e := ws.WriteHeader(c.Conn, wsH)
			if e != nil {
				return 0, e
			}
			return utils.BuffersWriteTo(buffers, c.Conn)
			/*
				实测使用writev并没有太大速度提升，反到速度不稳定, 而我们自己的函数是非常稳定的
				net.Buffers.WriteTo  (writev)
					2667 2226
					2689 2424
					2677 2412
					2924 2409
					2876 2413
					2398 2393

				utils.BuffersWriteTo
					2747 2378
					2831 2419
					2800 2413
					2802 2404

			*/
		}

	} else {

		bigbs, dup := utils.MergeBuffers(buffers)
		n, e := c.Write(bigbs)
		if dup {
			utils.PutPacket(bigbs)

		}
		return int64(n), e
	}
}

// Write websocket binary frames
func (c *Conn) Write(p []byte) (n int, e error) {

	//查看了代码，wsutil.WriteClientBinary 等类似函数会直接调用 ws.WriteFrame， 是不分片的.
	// 不分片的效率更高,因为无需缓存,zero copy

	if c.state == ws.StateClientSide {
		e = wsutil.WriteClientBinary(c.Conn, p) //实际我查看它的代码，发现Client端 最终调用到的 writeFrame 函数 还是多了一次拷贝; 它是为了防止篡改客户数据；但是我们代理的话不会使用数据，只是转发而已
	} else {
		e = wsutil.WriteServerBinary(c.Conn, p)
	}

	if e == nil {
		n = len(p)
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
		}
		//log.Println("ws Write finish", n, e)
		return
	*/

}

func (c *Conn) RemoteAddr() net.Addr {
	if c.realRaddr != nil {
		return c.realRaddr
	}
	return c.Conn.RemoteAddr()
}
