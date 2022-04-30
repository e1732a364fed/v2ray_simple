package utils

import "io"

// bufio.Reader 和 bytes.Buffer 都实现了 ByteReader
type ByteReader interface {
	ReadByte() (byte, error)
	Read(p []byte) (n int, err error)
}

// bytes.Buffer 实现了 ByteWriter
type ByteWriter interface {
	WriteByte(byte) error
	Write(p []byte) (n int, err error)
}

type ReadWrapper struct {
	io.Reader
	OptionalReader    io.Reader
	RemainFirstBufLen int
}

func (rw *ReadWrapper) Read(p []byte) (n int, err error) {

	if rw.RemainFirstBufLen > 0 {
		n, err := rw.OptionalReader.Read(p)
		if n > 0 {
			rw.RemainFirstBufLen -= n
		}
		return n, err
	} else {
		return rw.Reader.Read(p)
	}

}
func (rw *ReadWrapper) Close() error {
	if cc := rw.Reader.(io.Closer); cc != nil {
		return cc.Close()
	}
	return nil
}

type DummyReadCloser struct{}

//return 0, io.EOF
func (DummyReadCloser) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

//return nil
func (DummyReadCloser) Close() error {
	return nil
}
