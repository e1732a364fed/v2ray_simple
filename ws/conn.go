package ws

import (
	"io"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/utils"
)

// 因为 gobwas/ws 不包装conn，在写入和读取二进制时需要使用 wsutil包的特殊函数才行，
// 因此我们包装一下，统一使用Read和Write函数 来读写 二进制数据。因为我们这里是代理，
// 所以我们默认 抛弃 websocket的 数据帧 长度。
// 如果以后考虑与 vless v1的 udp 相结合的话，则数据包长度 不能丢弃，需要使用另外的实现。
type Conn struct {
	net.Conn
	first_nextFrameCalled bool

	state ws.State
	r     *wsutil.Reader
	w     *wsutil.Writer

	remainLenForLastFrame int64

	//lastReadFrameIsFragmented bool
}

//读取binary
func (c *Conn) Read(p []byte) (int, error) {

	//websocket 协议中帧长度上限为2^64，超大，考虑到我们vs的标准Packet缓存是64k，也就是2^16,
	// https://www.rfc-editor.org/rfc/rfc6455#section-5.2
	// (使用了 Extended payload length 字段)
	// 肯定会有多读的情况，此时如果一次用 wsutil.ReadServerBinary()的话，那么服务器缓存就超大，不可能如此实现
	// （wsutil.ReadServerBinary内部使用了 io.ReadAll, 而ReadAll是会无限增长内存的
	// 所以我们肯定要分段读， 直接用 wsutil.Reader.Read 即可， 但是第一个Read前必须要有 NextFrame调用
	//

	if c.remainLenForLastFrame > 0 {
		//log.Println("c.remainLenForLastFrame > 0", c.remainLenForLastFrame)
		n, e := c.r.Read(p)
		//log.Println("c.remainLenForLastFrame > 0, read ok", n, e, c.remainLenForLastFrame-int64(n))
		if e != nil && e != io.EOF {
			return n, e
		}
		c.remainLenForLastFrame -= int64(n)
		return n, nil
	}

	//log.Println("Reading next frame header")
	h, e := c.r.NextFrame()
	if e != nil {
		return 0, e
	}
	if h.OpCode != ws.OpBinary {
		//我们代理 只传输二进制格式

		return 0, utils.NewDataErr("ws first OpCode not Binary", nil, h.OpCode)
	}
	//log.Println("Read next frame header ok,", h.Length, c.r.State.Fragmented(), "givenbuf len", len(p))

	//c.lastReadFrameIsFragmented = c.r.State.Fragmented()

	c.remainLenForLastFrame = h.Length

	//r.NextFrame 内部会使用一个 io.LimitedReader, 每次Read到的最大长度为一个Frame首部所标明的大小
	// io.LimitedReader 最后一次Read，是会返回EOF的， 但是这种正常的EOF已经被 wsutil.Reader.Read处理了

	// 见 https://github.com/gobwas/ws/blob/2effe5ec7e4fd737085eb56beded3ad6491269dc/wsutil/reader.go#L125

	// 但是后来发现，只有 fragmented的情况下，才会处理EOF，否则还是会传递到我们这里

	// 也就是说，websocket虽然一个数据帧可以超大，但是 还有一种 分片功能，而如果不分片的话，gobwas就不处理EOF

	//经过实测，如果数据比较小的话，就不会分片，此时就会出现EOF; 如果数据比较大，比如 4327，就要分片

	// 不过后面发现，在分片时，读取到了整个数据后，会出问题，导致客户端没法读到实际数据，不知何故
	//  所以暂时先在Write端 把分片功能关闭。 （DisableFlush)

	n, e := c.r.Read(p)
	//log.Println("read data result", e, n)

	c.remainLenForLastFrame -= int64(n)

	if e != nil && e != io.EOF {

		//log.Println("e", e, n, string(p[:n]), p[:n])
		return n, e

	}
	return n, nil
}

//Write binary
func (c *Conn) Write(p []byte) (n int, e error) {
	//log.Println("ws Write called", len(p))
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
}
