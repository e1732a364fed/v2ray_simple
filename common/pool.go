package common

import (
	"bytes"
	"sync"
)

var (
	standardBytesPool  sync.Pool
	standardPacketPool sync.Pool
	customBytesPool    sync.Pool

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

func GetPacket() []byte {
	return standardPacketPool.Get().([]byte)
}

func PutPacket(bs []byte) {
	c := cap(bs)
	if c < maxBufLen {
		if c > StandardBytesLength {
			standardBytesPool.Put(bs[:c])
		}
		return
	}

	standardPacketPool.Put(bs[:c])
}

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
