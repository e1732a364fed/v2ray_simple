package netLayer

import (
	"io"
	"net"
	"os"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 选择性从 OptionalReader读取, 直到 RemainFirstBufLen 小于等于0 为止；
type ReadWrapper struct {
	net.Conn
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
		return rw.Conn.Read(p)
	}

}

func (rw *ReadWrapper) WriteBuffers(buffers [][]byte) (int64, error) {
	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := rw.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}

// 一个自定义的由多个组件组成的实现 net.Conn 的结构, 也通过设置 Rejecter 实现 RejectConn
type IOWrapper struct {
	EasyNetAddresser
	EasyDeadline //无需再调用 InitEasyDeadline，内部已经处理好了。

	io.Reader //不可为nil
	io.Writer //不可为nil
	io.Closer

	FirstWriteChan chan struct{} //用于确保先Write然后再Read，可为nil

	CloseChan chan struct{} //可为nil，用于接收关闭信号

	deadlineInited bool

	closeOnce, firstWriteOnce sync.Once

	Rejecter RejectConn
}

func (iw *IOWrapper) Read(p []byte) (int, error) {
	if !iw.deadlineInited {
		iw.InitEasyDeadline()
		iw.deadlineInited = true
	}
	select {
	case <-iw.ReadTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		if iw.FirstWriteChan != nil {
			<-iw.FirstWriteChan
			return iw.Reader.Read(p)
		} else {
			return iw.Reader.Read(p)

		}
	}
}

func (iw *IOWrapper) Write(p []byte) (int, error) {

	if iw.FirstWriteChan != nil {
		defer iw.firstWriteOnce.Do(func() {
			close(iw.FirstWriteChan)
		})

	}

	if !iw.deadlineInited {
		iw.InitEasyDeadline()
		iw.deadlineInited = true
	}
	select {
	case <-iw.WriteTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		return iw.Writer.Write(p)
	}
}

func (iw *IOWrapper) Close() error {
	if c := iw.Closer; c != nil {
		return c.Close()
	}
	if iw.CloseChan != nil {
		iw.closeOnce.Do(func() {
			close(iw.CloseChan)
		})

	}
	return nil
}

func (iw *IOWrapper) RejectBehaviorDefined() bool {

	return iw.Rejecter != nil && iw.Rejecter.RejectBehaviorDefined()
}
func (iw *IOWrapper) Reject() {
	if iw.Rejecter != nil {
		iw.Rejecter.Reject()
	}
}
