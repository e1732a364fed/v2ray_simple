//go:build !windows && !wasm && !illumos
// +build !windows,!wasm,!illumos

package utils

import (
	"syscall"
	"unsafe"
)

func GetReadVReader() SystemReadver {
	return &posixReader{}
}

type posixReader struct {
	iovecs []syscall.Iovec
}

func (r *posixReader) Init(bs [][]byte, singleBufLen int) {
	iovecs := r.iovecs
	if iovecs == nil {
		iovecs = make([]syscall.Iovec, 0, len(bs))
	}
	for idx, b := range bs {
		iovecs = append(iovecs, syscall.Iovec{
			Base: &(b[0]),
		})
		iovecs[idx].SetLen(singleBufLen)
	}
	r.iovecs = iovecs
}

//正常readv返回的应该是 ssize_t, 在64位机器上应该是 int64, 但是负数只用于返回-1错误，而且我们提供的buffer长度远远小于 uint32的上限；所以uint32可以
func (r *posixReader) Read(fd uintptr) (uint32, error) {
	n, _, e := syscall.Syscall(syscall.SYS_READV, fd, uintptr(unsafe.Pointer(&r.iovecs[0])), uintptr(len(r.iovecs)))
	if e != 0 {
		return 0, e
	}
	return uint32(n), nil
}

func (r *posixReader) Clear() {
	for idx := range r.iovecs {
		r.iovecs[idx].Base = nil
	}
	r.iovecs = r.iovecs[:0]
}
