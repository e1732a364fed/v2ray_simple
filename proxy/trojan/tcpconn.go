package trojan

import (
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// trojan比较简洁，这个 UserTCPConn 只是用于读取握手读取时读到的剩余的缓存。
// 实现 net.Conn, io.ReaderFrom, utils.User, utils.MultiWriter, netLayer.Splicer, netLayer.ConnWrapper
type UserTCPConn struct {
	net.Conn
	User
	optionalReader io.Reader //在使用了缓存读取握手包头后，就产生了buffer中有剩余数据的可能性，此时就要使用MultiReader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	isServerEnd bool

	mw utils.MultiWriter
}

func (c *UserTCPConn) Upstream() net.Conn {
	return c.Conn
}

func (c *UserTCPConn) Read(p []byte) (int, error) {
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
func (c *UserTCPConn) Write(p []byte) (int, error) {
	return c.Conn.Write(p)
}

// 当底层链接可以暴露为 tcp或 unix链接时，返回true
func (c *UserTCPConn) EverPossibleToSpliceRead() bool {
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

func (c *UserTCPConn) CanSpliceRead() (bool, *net.TCPConn, *net.UnixConn) {
	if c.isServerEnd {
		if c.remainFirstBufLen > 0 {
			return false, nil, nil
		}
	}

	return netLayer.ReturnSpliceRead(c.Conn)
}

func (c *UserTCPConn) EverPossibleToSpliceWrite() bool {

	if netLayer.IsTCP(c.Conn) != nil {
		return true
	}
	if s, ok := c.Conn.(netLayer.Splicer); ok {
		return s.EverPossibleToSpliceWrite()
	}
	return false
}

func (c *UserTCPConn) CanSpliceWrite() (r bool, conn *net.TCPConn) {
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
func (c *UserTCPConn) ReadFrom(r io.Reader) (written int64, err error) {

	return netLayer.TryReadFrom_withSplice(c, c.Conn, r, func() bool { return c.isServerEnd || c.remainFirstBufLen <= 0 })
}

func (c *UserTCPConn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.isServerEnd || c.remainFirstBufLen <= 0 {

		//底层连接可以是 ws，/ tls / 基本连接; tls 我们暂不支持 utils.MultiWriter
		// 理论上tls是可以支持的，但是要我们魔改tls库

		//本作的 ws.Conn 实现了 utils.MultiWriter

		if c.underlayIsBasic {
			return utils.BuffersWriteTo(buffers, c.Conn)

		} else if c.mw != nil {
			return c.mw.WriteBuffers(buffers)
		}
	}

	//发现用tls时，下面的 MergeBuffers然后一起写入的方式，能提供巨大的性能提升

	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := c.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}
