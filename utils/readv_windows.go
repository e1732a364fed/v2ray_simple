package utils

import (
	"syscall"
)

func GetReadVReader() MultiReader {
	return new(windowsReader)
}

type windowsReader struct {
	bufs []syscall.WSABuf
}

func (r *windowsReader) Init(bs [][]byte) {
	if r.bufs == nil {
		r.bufs = make([]syscall.WSABuf, 0, len(bs))
	}
	for _, b := range bs {
		r.bufs = append(r.bufs, syscall.WSABuf{Len: uint32(StandardBytesLength), Buf: &b[0]})
	}
}

func (r *windowsReader) Clear() {
	for idx := range r.bufs {
		r.bufs[idx].Buf = nil
	}
	r.bufs = r.bufs[:0]
}

func (r *windowsReader) Read(fd uintptr) (uint32, error) {
	var nBytes uint32
	var flags uint32
	err := syscall.WSARecv(syscall.Handle(fd), &r.bufs[0], uint32(len(r.bufs)), &nBytes, &flags, nil, nil)
	if err != nil {
		return 0, err
	}
	return nBytes, nil
}
