package simplesocks

import (
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 实现 utils.User, utils.UserAssigner
type TCPConn struct {
	net.Conn
	optionalReader io.Reader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	isServerEnd bool

	upstreamUser utils.User
}

// 实现 utils.UserAssigner
func (c *TCPConn) SetUser(u utils.User) {
	c.upstreamUser = u
}

func (c *TCPConn) IdentityStr() string {
	if c.upstreamUser != nil {
		return c.upstreamUser.IdentityStr()
	}
	return ""
}

func (c *TCPConn) IdentityBytes() []byte {
	if c.upstreamUser != nil {
		return c.upstreamUser.IdentityBytes()
	}
	return nil
}

func (c *TCPConn) AuthStr() string {
	if c.upstreamUser != nil {
		return c.upstreamUser.AuthStr()
	}
	return ""
}
func (c *TCPConn) AuthBytes() []byte {
	if c.upstreamUser != nil {
		return c.upstreamUser.AuthBytes()
	}
	return nil
}

func (c *TCPConn) Upstream() net.Conn {
	return c.Conn
}

// 当底层链接可以暴露为 tcp或 unix链接时，返回true
func (c *TCPConn) EverPossibleToSpliceRead() bool {
	if netLayer.IsTCP(c.Conn) != nil {
		return true
	}
	if netLayer.IsUnix(c.Conn) != nil {
		return true
	}

	if s, ok := c.Conn.(netLayer.SpliceReader); ok {
		return s.EverPossibleToSpliceRead()
	}

	return false
}

func (c *TCPConn) CanSpliceRead() (bool, *net.TCPConn, *net.UnixConn) {
	if c.isServerEnd {
		if c.remainFirstBufLen > 0 {
			return false, nil, nil
		}
	}

	return netLayer.ReturnSpliceRead(c.Conn)
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

func (c *TCPConn) EverPossibleToSpliceWrite() bool {

	if netLayer.IsTCP(c.Conn) != nil {
		return true
	}
	if s, ok := c.Conn.(netLayer.Splicer); ok {
		return s.EverPossibleToSpliceWrite()
	}
	return false
}

func (c *TCPConn) CanSpliceWrite() (r bool, conn *net.TCPConn) {
	if !c.isServerEnd && c.remainFirstBufLen > 0 {
		return false, nil
	}

	if tc := netLayer.IsTCP(c.Conn); tc != nil {
		r = true
		conn = tc

	} else if s, ok := c.Conn.(netLayer.Splicer); ok {
		r, conn = s.CanSpliceWrite()
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
