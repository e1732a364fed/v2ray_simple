package tlsLayer

import (
	"encoding/binary"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 读写都按tls application data 的格式走，其实就是加个包头. 实现 utils.MultiWriter
type FakeAppDataConn struct {
	net.Conn
	readRemaining int

	OptionalReader          io.Reader
	OptionalReaderRemainLen int
}

func (c *FakeAppDataConn) Read(p []byte) (n int, err error) {
	if c.OptionalReaderRemainLen > 0 {
		n, err := c.OptionalReader.Read(p)
		if n > 0 {
			c.OptionalReaderRemainLen -= n
		}
		return n, err
	}

	if c.readRemaining > 0 {
		if len(p) > c.readRemaining {
			p = p[:c.readRemaining]
		}
		n, err = c.Conn.Read(p)
		c.readRemaining -= n
		return
	}
	var tlsHeader [5]byte
	_, err = io.ReadFull(c.Conn, tlsHeader[:])
	if err != nil {
		return
	}
	length := int(binary.BigEndian.Uint16(tlsHeader[3:]))
	if tlsHeader[0] != 23 {
		return 0, utils.ErrInErr{ErrDesc: "unexpected TLS record type: ", Data: tlsHeader[0]}
	}
	readLen := len(p)
	if readLen > length {
		readLen = length
	}
	n, err = c.Conn.Read(p[:readLen])
	if err != nil {
		return
	}
	c.readRemaining = length - n
	return
}

func (c *FakeAppDataConn) Write(p []byte) (n int, err error) {

	const maxlen = 1 << 14
	var nn int

	for len(p) > maxlen {
		nn, err = WriteAppDataNoBuf(c.Conn, p[:maxlen])

		n += nn

		if err != nil {
			return
		}
		p = p[maxlen:]

	}

	nn, err = WriteAppDataNoBuf(c.Conn, p)

	n += nn

	return
}

func (c *FakeAppDataConn) Upstream() any {
	return c.Conn
}

func (c *FakeAppDataConn) WriteBuffers(bss [][]byte) (int64, error) {
	// 在server端，从direct用readv读到的数据可以用 WriteBuffers写回，可以加速

	allLen := utils.BuffersLen(bss)
	err := WriteAppDataHeader(c.Conn, allLen)
	if err != nil {
		return 0, err
	}
	return utils.BuffersWriteTo(bss, c.Conn)
}
