package simplesocks

import (
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

type TCPConn struct {
	net.Conn
	optionalReader io.Reader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	isServerEnd bool
}

func (c *TCPConn) Upstream() net.Conn {
	return c.Conn
}

func (c *TCPConn) Read(p []byte) (int, error) {
	if c.remainFirstBufLen > 0 {
		n, err := c.optionalReader.Read(p)
		if n > 0 {
			c.remainFirstBufLen -= n
		}
		return n, err
	} else {
		return c.Conn.Read(p)
	}
}
func (c *TCPConn) Write(p []byte) (int, error) {
	return c.Conn.Write(p)
}

func (c *TCPConn) EverPossibleToSplice() bool {

	if netLayer.IsTCP(c.Conn) != nil {
		return true
	}
	if s, ok := c.Conn.(netLayer.Splicer); ok {
		return s.EverPossibleToSplice()
	}
	return false
}

func (c *TCPConn) CanSplice() (r bool, conn *net.TCPConn) {
	if !c.isServerEnd && c.remainFirstBufLen > 0 {
		return false, nil
	}

	if tc := netLayer.IsTCP(c.Conn); tc != nil {
		r = true
		conn = tc

	} else if s, ok := c.Conn.(netLayer.Splicer); ok {
		r, conn = s.CanSplice()
	}

	return
}
func (c *TCPConn) ReadFrom(r io.Reader) (written int64, err error) {

	return netLayer.TryReadFrom_withSplice(c, c.Conn, r, func() bool { return c.isServerEnd || c.remainFirstBufLen <= 0 })
}

func (c *TCPConn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.isServerEnd || c.remainFirstBufLen <= 0 {

		if c.underlayIsBasic {
			return utils.BuffersWriteTo(buffers, c.Conn)

		} else if mr, ok := c.Conn.(utils.MultiWriter); ok {
			return mr.WriteBuffers(buffers)
		}
	}

	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := c.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}
