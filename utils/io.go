package utils

import (
	"io"
	"sync"
)

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

//optionally read from OptionalReader
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
//if ReadCount<0, return 0, io.EOF
func (d *DummyReadCloser) Read(p []byte) (int, error) {
	d.ReadCount -= 1
	//log.Println("read called", d.ReadCount)
	if d.ReadCount < 0 {
		return 0, io.EOF

	} else {
		return len(p), nil
	}
}

//return nil
func (DummyReadCloser) Close() error {
	return nil
}

type DummyWriteCloser struct {
	WriteCount int
}

// WriteCount -= 1 at each call.
//if WriteCount<0, return 0, io.EOF
func (d *DummyWriteCloser) Write(p []byte) (int, error) {
	d.WriteCount -= 1
	//log.Println("write called", d.WriteCount)

	if d.WriteCount < 0 {
		return 0, io.EOF

	} else {
		return len(p), nil
	}
}

//return nil
func (DummyWriteCloser) Close() error {
	return nil
}

//先从Old读，若SwitchChan被关闭, 立刻改为从New读
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

//先向Old写，若SwitchChan被关闭, 改向New写
type WriteSwitcher struct {
	Old, New   io.Writer     //non-nil
	SwitchChan chan struct{} //non-nil
	io.Closer
}

func (d *WriteSwitcher) Write(p []byte) (int, error) {

	select {
	case <-d.SwitchChan:
		return d.New.Write(p)

	default:
		return d.Old.Write(p)
	}
}

func (d *WriteSwitcher) Close() error {
	if d.Closer != nil {
		return d.Closer.Close()
	}
	return nil
}

//simple structure that send a signal by chan when Close called.
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
