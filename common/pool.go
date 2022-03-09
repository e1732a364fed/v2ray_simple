package common

import (
	"sync"
)

var standardBytesPool sync.Pool

var customBytesPool sync.Pool

const standardBytesLength int = 1500

func init() {
	standardBytesPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, standardBytesLength)
		},
	}
}

func GetBytes(size int) []byte {
	if size < standardBytesLength {
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
	if c < standardBytesLength {

		return
	} else if c == standardBytesLength {
		standardBytesPool.Put(bs[:c])
	} else {
		customBytesPool.Put(bs)
	}
}
