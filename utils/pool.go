package utils

import (
	"bytes"
	"sync"
)

var (
	mtuPool sync.Pool //专门储存 长度为 MTU 的 *[]byte, 注意这里存储的是指针

	// packetPool 专门储存 长度为 MaxPacketLen 的 []byte
	//
	// 参考对比: tcp默认是 16k，范围是1k～128k;
	// 而 udp则最大还不到 64k (65535－20－8);
	// io.Copy 内部默认buffer大小为 32k;
	// 总之 我们64k已经够了
	packetPool sync.Pool

	bufPool sync.Pool //储存 *bytes.Buffer
)

const (
	//即 Maximum transmission unit, 参照的是 Ethernet v2 的MTU;
	MTU int = 1500

	//注意wifi信号MTU是 2304，我们并未考虑wifi,主要是因为就算用wifi传, 早晚还是要经过以太网,除非两个wifi设备互传
	// https://en.wikipedia.org/wiki/Maximum_transmission_unit

	//本作设定的最大包 长度大小，64k
	MaxPacketLen = 64 * 1024
)

func init() {

	mtuPool = sync.Pool{
		New: func() interface{} {
			bs := make([]byte, MTU)
			return &bs
		},
	}

	packetPool = sync.Pool{
		New: func() interface{} {
			bs := make([]byte, MaxPacketLen)
			return &bs
		},
	}

	bufPool = sync.Pool{
		New: func() interface{} {
			return new(bytes.Buffer)
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
	bsPtr := packetPool.Get().(*[]byte)
	returnValue := *bsPtr
	return returnValue[:MaxPacketLen]
}

// 放回用 GetPacket 获取的 []byte
func PutPacket(bs []byte) {
	c := cap(bs)
	if c < MaxPacketLen {
		if c >= MTU {
			bs = bs[:MTU]
			mtuPool.Put(&bs)
		}
		return
	}

	bs = bs[:MaxPacketLen]
	packetPool.Put(&bs)
}

// 从Pool中获取一个 MTU 长度的 []byte
func GetMTU() []byte {
	bs := mtuPool.Get().(*[]byte)
	returnValue := *bs
	return returnValue[:MTU]
}

// 从pool中获取 []byte, 根据给出长度不同，来源于的Pool会不同.
func GetBytes(size int) []byte {
	if size <= MTU {
		bsPtr := mtuPool.Get().(*[]byte)
		bs := *bsPtr
		return bs[:size]
	}

	return GetPacket()[:size]

}

// 根据bs长度 选择放入各种pool中, 只有 cap(bs)>=MTU 才会被处理
func PutBytes(bs []byte) {
	c := cap(bs)
	if c < MTU {

		return
	} else if c < MaxPacketLen {

		bs = bs[:MTU]
		mtuPool.Put(&bs)
	} else {
		bs = bs[:MaxPacketLen]
		packetPool.Put(&bs)
	}
}
