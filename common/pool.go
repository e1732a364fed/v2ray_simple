package common

import (
	"bytes"
	"sync"
)

var (
	standardBytesPool sync.Pool //1500

	// 实际上tcp默认是 16384, 16k，实际上范围是1k～128k之间，我们64k已经够了
	//而 udp则最大还不到 64k。(65535－20－8)
	standardPacketPool sync.Pool //64*1024

	customBytesPool sync.Pool // >1500

	bufPool sync.Pool
)

const StandardBytesLength int = 1500
const maxBufLen int = 64 * 1024

func init() {
	standardBytesPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, StandardBytesLength)
		},
	}

	standardPacketPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, maxBufLen)
		},
	}

	customBytesPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, maxBufLen)
		},
	}

	bufPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
}

func GetBuf() *bytes.Buffer {
	return bufPool.Get().(*bytes.Buffer)
}

func PutBuf(buf *bytes.Buffer) {
	buf.Reset()
	bufPool.Put(buf)
}

//建议在 Read net.Conn 时, 使用 GetPacket函数 获取到足够大的byte（64*1024字节）
func GetPacket() []byte {
	return standardPacketPool.Get().([]byte)
}

func PutPacket(bs []byte) {
	c := cap(bs)
	if c < maxBufLen { //如果不够大，考虑放到更小的 pool里
		if c > StandardBytesLength {
			standardBytesPool.Put(bs[:c])
		}
		return
	}

	standardPacketPool.Put(bs[:c])
}

// 从pool中获取 []byte, 在 size <= 1500时有最佳性能
func GetBytes(size int) []byte {
	if size < StandardBytesLength {
		bs := standardBytesPool.Get().([]byte)
		return bs[:size]
	}

	randomBytes1 := standardBytesPool.Get().([]byte)

	if len(randomBytes1) >= size {
		return randomBytes1[:size]
	} else {
		standardBytesPool.Put(randomBytes1)
		return make([]byte, size)
	}

}

func PutBytes(bs []byte) {
	c := cap(bs)
	if c < StandardBytesLength {

		return
	} else if c == StandardBytesLength {
		standardBytesPool.Put(bs[:c])
	} else {
		customBytesPool.Put(bs)
	}
}
