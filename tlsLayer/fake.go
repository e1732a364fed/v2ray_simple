package tlsLayer

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 读写都按tls application data 的格式走，其实就是加个包头
type FakeAppDataConn struct {
	net.Conn
	readRemaining int

	//for readv
	rr syscall.RawConn
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

func (c *FakeAppDataConn) WillReadBuffersBenifit() int {
	//一般而言底层连接就是tcp
	if r, rr := netLayer.IsConnGoodForReadv(c.Conn); r != 0 {
		c.rr = rr
	}

	return 1
}

// ReadBuffers一次性尽可能多读几个buf, 每个buf内部都是一个完整的appdata
func (c *FakeAppDataConn) ReadBuffers() (buffers [][]byte, err error) {

	//最理想的情况是，一次性读到一个大包，我们根据tls包头分割出多个appdata，然后放入buffers中

	//但是有可能读不完整

	var reader io.Reader

	wholeReadLen := 0
	//一次性读取
	if c.rr != nil {
		readv_mem := utils.Get_readvMem()
		buffers, err = utils.ReadvFrom(c.rr, readv_mem)
		if err != nil {
			return nil, err
		}

		reader = utils.BuffersToMultiReader(buffers)
		wholeReadLen = utils.BuffersLen(buffers)
	} else {
		bs := utils.GetPacket()
		wholeReadLen, err = c.Conn.Read(bs)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewBuffer(bs[:wholeReadLen])
	}
	if wholeReadLen < 5 {
		return nil, errors.New("FakeAppDataConn ReadBuffers too short")
	}
	wholeLeftLen := wholeReadLen

	if c.readRemaining > 0 {
		bs := utils.GetPacket()
		var n int
		n, err = c.Conn.Read(bs[:c.readRemaining])
		c.readRemaining -= n
		buffers = append(buffers, bs[:n])
		return
	}
	for {
		if wholeLeftLen < 5 {
			reader = io.MultiReader(reader, c.Conn)
			var tlsHeader [5]byte
			_, err = io.ReadFull(reader, tlsHeader[:])
			if err != nil {
				return
			}
			thisAppDataLen := int(binary.BigEndian.Uint16(tlsHeader[3:]))
			if tlsHeader[0] != 23 {
				return nil, utils.ErrInErr{ErrDesc: "unexpected TLS record type: ", Data: tlsHeader[0]}
			}
			thisBs := utils.GetPacket()

			var n int
			n, err = reader.Read(thisBs[:thisAppDataLen])
			if err != nil {
				return
			}
			buffers = append(buffers, thisBs[:n])

			if diff := thisAppDataLen - n; diff > 0 {
				c.readRemaining = diff

			} else if diff < 0 {
				return nil, utils.ErrInErr{ErrDesc: "FakeAppDataConn ReadBuffers read n>thisAppDataLen ", Data: []int{n, thisAppDataLen}}
			}
			return

		}

		var tlsHeader [5]byte
		_, err = io.ReadFull(reader, tlsHeader[:])
		if err != nil {
			return
		}
		wholeLeftLen -= 5

		thisAppDataLen := int(binary.BigEndian.Uint16(tlsHeader[3:]))
		if tlsHeader[0] != 23 {
			return nil, utils.ErrInErr{ErrDesc: "unexpected TLS record type: ", Data: tlsHeader[0]}
		}
		thisBs := utils.GetPacket()

		var n int
		n, err = reader.Read(thisBs[:thisAppDataLen])
		if err != nil {
			return
		}
		buffers = append(buffers, thisBs[:n])

		if diff := thisAppDataLen - n; diff > 0 {
			c.readRemaining = diff
			return

		} else if diff < 0 {
			return nil, utils.ErrInErr{ErrDesc: "FakeAppDataConn ReadBuffers read n>thisAppDataLen ", Data: []int{n, thisAppDataLen}}
		}

		wholeLeftLen -= thisAppDataLen

		if wholeLeftLen == 0 {
			return
		}

	}

}
func (c *FakeAppDataConn) PutBuffers(bss [][]byte) {
	for _, v := range bss {
		utils.PutPacket(v)
	}

}
