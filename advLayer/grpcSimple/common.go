package grpcSimple

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"time"

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

type commonReadPart struct {
	remain int
	br     *bufio.Reader
}

func (*commonReadPart) LocalAddr() net.Addr  { return nil }
func (*commonReadPart) RemoteAddr() net.Addr { return nil }

func (g *commonReadPart) Read(b []byte) (n int, err error) {

	if g.remain > 0 {

		size := g.remain
		if len(b) < size {
			size = len(b)
		}

		n, err = io.ReadFull(g.br, b[:size])
		g.remain -= n
		return
	}

	_, err = g.br.Discard(6)
	if err != nil {

		return 0, err
	}

	protobufPayloadLen, err := binary.ReadUvarint(g.br)
	if err != nil {
		return 0, ErrInvalidLength
	}

	size := int(protobufPayloadLen)
	if len(b) < size {
		size = len(b)
	}

	n, err = io.ReadFull(g.br, b[:size])
	if err != nil {
		return
	}

	remain := int(protobufPayloadLen) - n
	if remain > 0 {
		g.remain = remain
	}
	return n, nil
}

type timeouter struct {
	deadline *time.Timer

	closeFunc func()
}

func (g *timeouter) SetReadDeadline(t time.Time) error  { return g.SetDeadline(t) }
func (g *timeouter) SetWriteDeadline(t time.Time) error { return g.SetDeadline(t) }

func (g *timeouter) SetDeadline(t time.Time) error {

	var d time.Duration

	if g.deadline != nil {

		if t == (time.Time{}) {
			g.deadline.Stop()
			return nil
		}

		g.deadline.Reset(d)
		return nil
	} else {
		if t == (time.Time{}) {
			return nil
		}
		d = time.Until(t)

	}

	g.deadline = time.AfterFunc(d, g.closeFunc)
	return nil
}
