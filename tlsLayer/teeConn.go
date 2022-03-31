package tlsLayer

import (
	"bytes"
	"io"
	"log"
	"net"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

// 和TeeConn配合的是Recorder, 每次调用Write就会记录一个新Buffer
type Recorder struct {
	Buflist    []*bytes.Buffer
	writeCount int
	stop       bool
}

func NewRecorder() *Recorder {
	return &Recorder{
		Buflist: make([]*bytes.Buffer, 0, 10),
	}
}

func (wr *Recorder) GetLast() *bytes.Buffer {
	if len(wr.Buflist) == 0 {
		return nil
	}
	return wr.Buflist[len(wr.Buflist)-1]
}

//停止记录后，Write方法将不产生任何开销
func (wr *Recorder) StopRecord() {
	wr.stop = true
}

// StartRecord后，Recorder就会开始记录数据。默认Recorder就是开启状态；
//  所以此方法仅用于之前 Stop过
func (wr *Recorder) StartRecord() {
	wr.stop = false
}

//调用时，要保证目前没有任何人 正在 Write，否则会报错
func (wr *Recorder) ReleaseBuffers() {
	tmp := wr.Buflist
	wr.Buflist = nil //make([]*bytes.Buffer, 0, 10)

	for _, v := range tmp {
		utils.PutBuf(v)
	}

}

// 打印内部所有包的前10字节
func (wr *Recorder) DigestAll() {
	tmp := wr.Buflist

	for i, v := range tmp {
		bs := v.Bytes()
		min := 10
		if min > len(bs) {
			min = len(bs)
		}
		log.Println(i, len(bs), bs[:min])

	}

}

// 每Write一遍， 就写入一个新的buffer, 使用 utils.GetBuf() 获取
func (wr *Recorder) Write(p []byte) (n int, err error) {
	if wr.stop {
		return len(p), nil
	}

	if wr.writeCount > 0 { //舍弃第一个包，因为第一个包我们要做 tls的handshake检测 以及 vless的uuid检测，所以不可能没有tls的 数据包
		buf := utils.GetBuf()
		n, err = buf.Write(p)

		wr.Buflist = append(wr.Buflist, buf)
	} else {
		n = len(p)
	}

	wr.writeCount++
	if wr.writeCount > 50 { //不能无限记录，不然爆了
		wr.StopRecord()
	}
	return
}

// 实现net.Conn，专门用于 tls 检测步骤
//每次 Read TeeConn, 都会从OldConn进行Read，然后把Read到的数据同时写入 TargetWriter(NewTeeConn 的参数)
//
// 这个TeeConn设计时，专门用于 给 tls包一个 假的 net.Conn, 避免它 主动close我们的原Conn
//
// tls的Read是我们要关心的，Write则没有必要套Tee
type TeeConn struct {
	OldConn net.Conn

	TargetReader io.Reader
}

func NewTeeConn(oldConn net.Conn, targetWriter io.Writer) *TeeConn {

	return &TeeConn{
		OldConn:      oldConn,
		TargetReader: io.TeeReader(oldConn, targetWriter),
	}

}

// 使用我们的Tee功能进行Read
func (tc *TeeConn) Read(b []byte) (n int, err error) {

	n, err = tc.TargetReader.Read(b)
	//log.Println("TeeConn Read called", n)
	return
}

// 直接使用原Conn发送
func (tc *TeeConn) Write(b []byte) (n int, err error) {
	return tc.OldConn.Write(b)
}

//返回原Conn的地址
func (tc *TeeConn) LocalAddr() net.Addr {
	return tc.OldConn.LocalAddr()
}

//返回原Conn的地址
func (tc *TeeConn) RemoteAddr() net.Addr {
	return tc.OldConn.RemoteAddr()
}

func (tc *TeeConn) Close() error {
	tc.OldConn.Close()
	return nil
}

// 暂时先什么也不做。事实上，这里 如果deadline到期了 需要能够通知外界，
func (tc *TeeConn) SetDeadline(t time.Time) error {
	return nil
}

func (tc *TeeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (tc *TeeConn) SetWriteDeadline(t time.Time) error {
	return nil
}
