package tlsLayer

import (
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
	length := int(binary.BigEndian.Uint16(tlsHeader[3:5]))
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
	var header [5]byte

	header[0] = 23
	const maxlen = 16384
	for len(p) > maxlen {
		binary.BigEndian.PutUint16(header[1:3], tls.VersionTLS12)
		binary.BigEndian.PutUint16(header[3:5], uint16(maxlen))

		buf := utils.GetBuf()
		buf.Write(header[:])
		buf.Write(p[:maxlen])

		c.Conn.Write(buf.Bytes())
		utils.PutBuf(buf)

		if err != nil {
			return
		}
		n += maxlen
		p = p[maxlen:]
	}
	binary.BigEndian.PutUint16(header[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(header[3:5], uint16(len(p)))

	buf := utils.GetBuf()
	buf.Write(header[:])
	buf.Write(p)

	c.Conn.Write(buf.Bytes())
	utils.PutBuf(buf)

	if err == nil {
		n += len(p)
	}
	return
}

func (c *FakeAppDataConn) Upstream() any {
	return c.Conn
}
