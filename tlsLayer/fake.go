package tlsLayer

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 读写都按tls application data 的格式走，其实就是加个包头
type FakeAppDataConn struct {
	net.Conn
	readRemaining int
}

func (c *FakeAppDataConn) Read(p []byte) (n int, err error) {
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

func WriteAppData(conn io.Writer, buf *bytes.Buffer, d []byte) (n int, err error) {
	var h [5]byte
	h[0] = 23
	binary.BigEndian.PutUint16(h[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(h[3:], uint16(len(d)))

	shouldPut := false

	if buf == nil {
		buf = utils.GetBuf()
		shouldPut = true
	}
	buf.Write(h[:])
	buf.Write(d)

	n, err = conn.Write(buf.Bytes())

	if shouldPut {
		utils.PutBuf(buf)

	}
	return
}

// 一般conn直接为tcp连接，而它是有系统缓存的，因此我们一般不需要特地创建一个缓存
// 写两遍之后在发出
func WriteAppDataNoBuf(conn io.Writer, d []byte) (n int, err error) {
	var h [5]byte
	h[0] = 23
	binary.BigEndian.PutUint16(h[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(h[3:], uint16(len(d)))

	_, err = conn.Write(h[:])
	if err != nil {
		return
	}
	return conn.Write(d)

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
