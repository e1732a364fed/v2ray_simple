// Modified from: https://github.com/Dreamacro/clash/blob/master/transport/gun/gun.go
// License: MIT

package grpcSimple

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

func commonWrite(b []byte) *bytes.Buffer {
	protobufHeader := [binary.MaxVarintLen64 + 1]byte{0x0A}
	varuintSize := binary.PutUvarint(protobufHeader[1:], uint64(len(b)))
	grpcHeader := make([]byte, 5)
	grpcPayloadLen := uint32(varuintSize + 1 + len(b))
	binary.BigEndian.PutUint32(grpcHeader[1:5], grpcPayloadLen)

	buf := utils.GetBuf()

	buf.Write(grpcHeader)
	buf.Write(protobufHeader[:varuintSize+1])
	buf.Write(b)
	return buf
}

type commonPart struct {
	remain int
	br     *bufio.Reader

	la, ra net.Addr
}

//implements netLayer.RejectConn, return true
func (*commonPart) HasOwnDefaultRejectBehavior() bool {
	return true
}

func (c *commonPart) LocalAddr() net.Addr  { return c.la }
func (c *commonPart) RemoteAddr() net.Addr { return c.ra }

func (c *commonPart) Read(b []byte) (n int, err error) {

	if c.remain > 0 {

		size := c.remain
		if len(b) < size {
			size = len(b)
		}

		n, err = io.ReadFull(c.br, b[:size])
		c.remain -= n
		return
	}

	_, err = c.br.Discard(6)
	if err != nil {

		return 0, err
	}

	protobufPayloadLen, err := binary.ReadUvarint(c.br)
	if err != nil {
		return 0, utils.ErrInErr{ErrDesc: "Failed in grpc Read, binary.ReadUvarint", ErrDetail: err, ExtraIs: []error{utils.ErrInvalidData}}
	}

	size := int(protobufPayloadLen)
	if len(b) < size {
		size = len(b)
	}

	n, err = io.ReadFull(c.br, b[:size])
	if err != nil {
		return
	}

	remain := int(protobufPayloadLen) - n
	if remain > 0 {
		c.remain = remain
	}
	return n, nil
}

//timeouter 修改自 clash的gun.go, 是一种简单且不完美的deadline实现。
// clash的代码没有考虑到 设置 time.Time{} 空结构时要stop timer，我们加上了。
//
// timeouter一旦超时，就会直接关闭连接，这是无法恢复的。而且 读和写共用同一个deadline。
// 如果要求高的话，还是建议使用 netLayer.EasyDeadline.
// 但因为比较极简，所以我们保留了下来。

/*
type timeouter struct {
	deadline *time.Timer

	closeFunc func()
}

func (c *timeouter) SetReadDeadline(t time.Time) error  { return c.SetDeadline(t) }
func (c *timeouter) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }

func (c *timeouter) SetDeadline(t time.Time) error {

	var d time.Duration

	if c.deadline != nil {

		if t == (time.Time{}) {
			c.deadline.Stop()
			c.deadline = nil
			return nil
		}

		d = time.Until(t)
		c.deadline.Reset(d)

	} else {
		if t == (time.Time{}) {
			return nil
		}

		d = time.Until(t)
		c.deadline = time.AfterFunc(d, c.closeFunc)
	}

	return nil

}

*/
