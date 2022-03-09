package common

import (
	"sync"
)

var standardBytesPool sync.Pool

var customBytesPool sync.Pool

const StandardBytesLength int = 1500

func init() {
	standardBytesPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, StandardBytesLength)
		},
	}
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
