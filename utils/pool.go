package utils

import (
	"bytes"
	"flag"
	"sync"
)

var (
	standardBytesPool sync.Pool //专门储存 长度为 StandardBytesLength 的 []byte

	// 作为参考对比，tcp默认是 16384, 16k，实际上范围是1k～128k之间
	// 而 udp则最大还不到 64k。(65535－20－8)
	// io.Copy 内部默认buffer大小为 32k
	// 总之 我们64k已经够了
	standardPacketPool sync.Pool // 专门储存 长度为 MaxBufLen 的 []byte

	bufPool sync.Pool //储存 *bytes.Buffer
)

//即MTU, Maximum transmission unit, 参照的是 Ethernet v2 的MTU;
const StandardBytesLength int = 1500

//注意wifi信号MTU是 2304，我们并未考虑wifi,主要是因为就算用wifi传, 早晚还是要经过以太网,除非两个wifi设备互传
// https://en.wikipedia.org/wiki/Maximum_transmission_unit

//本作设定的最大buf大小，64k
var MaxBufLen = DefaultMaxBufLen

const DefaultMaxBufLen = 64 * 1024

func init() {
	flag.IntVar(&MaxBufLen, "bl", DefaultMaxBufLen, "buf len")

	standardBytesPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, StandardBytesLength)
		},
	}

	standardPacketPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, MaxBufLen)
		},
	}

	bufPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
}

//给了参数调节buf大小后,需要更新pool
func AdjustBufSize() {
	standardPacketPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, MaxBufLen)
		},
	}
}

//从Pool中获取一个 *bytes.Buffer
func GetBuf() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

//将 buf 放回 Pool
func PutBuf(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

//建议在 Read net.Conn 时, 使用 GetPacket函数 获取到足够大的 []byte（MaxBufLen）
func GetPacket() []byte {
	return standardPacketPool.Get().([]byte)
}

// 放回用 GetPacket 获取的 []byte
func PutPacket(bs []byte) {
	c := cap(bs)
	if c < MaxBufLen {
		if c >= StandardBytesLength {
			standardBytesPool.Put(bs[:StandardBytesLength])
		}
		return
	}

	standardPacketPool.Put(bs[:MaxBufLen])
}

// 从Pool中获取一个 StandardBytesLength 长度的 []byte
func GetMTU() []byte {
	return standardBytesPool.Get().([]byte)
}

// 从pool中获取 []byte, 根据给出长度不同，来源于的Pool会不同.
func GetBytes(size int) []byte {
	if size <= StandardBytesLength {
		bs := standardBytesPool.Get().([]byte)
		return bs[:size]
	}

	return GetPacket()[:size]

}

// 根据bs长度 选择放入各种pool中, 只有 cap(bs)>=1500 才会被处理
func PutBytes(bs []byte) {
	c := cap(bs)
	if c < StandardBytesLength {

		return
	} else if c >= StandardBytesLength && c < MaxBufLen {
		standardBytesPool.Put(bs[:StandardBytesLength])
	} else if c >= MaxBufLen {
		standardPacketPool.Put(bs[:MaxBufLen])
	}
}
