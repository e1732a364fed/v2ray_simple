package utils

import (
	"io"
	"sync"
)

// 摘自 io.CopyBuffer。 因为原始的 CopyBuffer会又调用ReadFrom, 如果splice调用的话会产生无限递归。
//
//	这里删掉了ReadFrom, 直接进行经典拷贝
func ClassicCopy(w io.Writer, r io.Reader) (written int64, err error) {
	bs := GetPacket()
	defer PutPacket(bs)
	for {

		nr, er := r.Read(bs)
		if nr > 0 {
			nw, ew := w.Write(bs[0:nr])

			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = ErrInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {

			err = er
			break
		}
	}
	return
}

// 一种简单的读写组合, 在ws包中被用到.
type RW struct {
	io.Reader
	io.Writer
}

type WriteWrapper interface {
	io.Writer

	GetRawWriter() io.Writer
	SetRawWriter(io.Writer)
}

// bufio.Reader and bytes.Buffer implemented ByteReader
type ByteReader interface {
	ReadByte() (byte, error)
	Read(p []byte) (n int, err error)
}

// bytes.Buffer implemented ByteWriter
type ByteWriter interface {
	WriteByte(byte) error
	Write(p []byte) (n int, err error)
}

// optionally read from OptionalReader
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

type DummyReadCloser struct {
	ReadCount int
}

// ReadCount -= 1 at each call.
// if ReadCount<0, return 0, io.EOF
func (d *DummyReadCloser) Read(p []byte) (int, error) {
	d.ReadCount -= 1
	//log.Println("read called", d.ReadCount)
	if d.ReadCount < 0 {
		return 0, io.EOF

	} else {
		return len(p), nil
	}
}

// return nil
func (DummyReadCloser) Close() error {
	return nil
}

type DummyWriteCloser struct {
	WriteCount int
}

// WriteCount -= 1 at each call.
// if WriteCount<0, return 0, io.EOF
func (d *DummyWriteCloser) Write(p []byte) (int, error) {
	d.WriteCount -= 1
	//log.Println("write called", d.WriteCount)

	if d.WriteCount < 0 {
		return 0, io.EOF

	} else {
		return len(p), nil
	}
}

// return nil
func (DummyWriteCloser) Close() error {
	return nil
}

// 先从Old读，若SwitchChan被关闭, 立刻改为从New读
type ReadSwitcher struct {
	Old, New   io.Reader     //non-nil
	SwitchChan chan struct{} //non-nil

	io.Closer

	readOnce sync.Once

	readOldChan chan []byte

	switched bool
}

func (d *ReadSwitcher) Read(p []byte) (int, error) {
	d.readOnce.Do(func() {
		d.readOldChan = make(chan []byte)
		go func() {

			for {
				pkt := GetPacket()
				n, err := d.Old.Read(pkt)
				if err != nil {
					close(d.readOldChan)
					break
				}
				pkt = pkt[:n]
				if !d.switched {
					d.readOldChan <- pkt
				} else {
					break
				}
			}
		}()
	})
	select {
	case <-d.SwitchChan:
		return d.New.Read(p)

	case data := <-d.readOldChan:
		n := copy(p, data)
		return n, nil
	}
}

func (d *ReadSwitcher) Close() error {
	if d.Closer != nil {
		return d.Closer.Close()
	}
	return nil
}

// 先向Old写，若SwitchChan被关闭, 改向New写
type WriteSwitcher struct {
	Old, New   io.Writer     //non-nil
	SwitchChan chan struct{} //non-nil
	io.Closer

	switched bool
}

func (d *WriteSwitcher) Write(p []byte) (int, error) {

	if !d.switched {
		select {
		case <-d.SwitchChan:
			d.switched = true
			return d.New.Write(p)

		default:
			return d.Old.Write(p)
		}
	} else {
		return d.New.Write(p)
	}

}

func (d *WriteSwitcher) Close() error {
	if d.Closer != nil {
		return d.Closer.Close()
	}
	return nil
}

// simple structure that send a signal by chan when Close called.
type ChanCloser struct {
	closeChan chan struct{}
	once      sync.Once
}

func NewChanCloser() (*ChanCloser, chan struct{}) {
	cc := make(chan struct{})
	return &ChanCloser{
		closeChan: cc,
	}, cc
}

func (cc *ChanCloser) Close() error {
	cc.once.Do(func() {
		close(cc.closeChan)
	})
	return nil
}

type MultiCloser struct {
	Closers []io.Closer
	sync.Once
}

func (cc *MultiCloser) Close() (result error) {
	cc.Once.Do(func() {
		var es Errs
		for i, c := range cc.Closers {
			e := c.Close()
			if e != nil {
				es.Add(ErrsItem{Index: i, E: e})
			}
		}
		if !es.OK() {
			result = es
		}
	})
	return
}

type SimpleCloser interface {
	Close()
}
type NilCloserWrapper struct {
	SimpleCloser
}

func (nc NilCloserWrapper) Close() error {
	nc.SimpleCloser.Close()
	return nil
}
