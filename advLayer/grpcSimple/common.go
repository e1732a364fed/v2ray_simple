// Modified from: https://github.com/Dreamacro/clash/blob/master/transport/gun/gun.go
// License: MIT

package grpcSimple

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
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
	netLayer.EasyDeadline

	remain int
	br     *bufio.Reader

	la, ra net.Addr
}

// implements netLayer.RejectConn, return true
func (*commonPart) HasOwnDefaultRejectBehavior() bool {
	return true
}

func (c *commonPart) LocalAddr() net.Addr  { return c.la }
func (c *commonPart) RemoteAddr() net.Addr { return c.ra }

func (c *commonPart) Read(b []byte) (n int, err error) {

	select {
	case <-c.ReadTimeoutChan():
		return 0, os.ErrDeadlineExceeded
	default:
		return c.read(b)
	}
}

func (c *commonPart) read(b []byte) (n int, err error) {

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
