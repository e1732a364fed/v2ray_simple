package trojan

import (
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//trojan比较简洁，这个 UserTCPConn 只是用于读取握手读取时读到的剩余的缓存
type UserTCPConn struct {
	net.Conn
	optionalReader io.Reader //在使用了缓存读取握手包头后，就产生了buffer中有剩余数据的可能性，此时就要使用MultiReader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	hash        string
	isServerEnd bool
}

func (uc *UserTCPConn) Read(p []byte) (int, error) {
	if uc.remainFirstBufLen > 0 {
		n, err := uc.optionalReader.Read(p)
		if n > 0 {
			uc.remainFirstBufLen -= n
		}
		return n, err
	} else {
		return uc.Conn.Read(p)
	}
}
func (uc *UserTCPConn) Write(p []byte) (int, error) {
	return uc.Conn.Write(p)
}

func (c *UserTCPConn) EverPossibleToSplice() bool {

	if netLayer.IsBasicConn(c.Conn) {
		return true
	}
	if s, ok := c.Conn.(netLayer.Splicer); ok {
		return s.EverPossibleToSplice()
	}
	return false
}

func (c *UserTCPConn) CanSplice() (r bool, conn net.Conn) {
	if !c.isServerEnd && c.remainFirstBufLen > 0 {
		return false, nil
	}

	if netLayer.IsBasicConn(c.Conn) {
		r = true
		conn = c.Conn

	} else if s, ok := c.Conn.(netLayer.Splicer); ok {
		r, conn = s.CanSplice()
	}

	return
}
func (c *UserTCPConn) ReadFrom(r io.Reader) (written int64, err error) {

	return netLayer.TryReadFrom_withSplice(c, c.Conn, r, func() bool { return c.isServerEnd || c.remainFirstBufLen <= 0 })
}

func (c *UserTCPConn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.isServerEnd || c.remainFirstBufLen <= 0 {

		//底层连接可以是 ws，或者 tls，或者 基本连接; tls 我们暂不支持 utils.MultiWriter
		// 理论上tls是可以支持的，但是要我们魔改tls库

		//本作的 ws.Conn 实现了 utils.MultiWriter

		if c.underlayIsBasic {
			return utils.BuffersWriteTo(buffers, c.Conn)

		} else if mr, ok := c.Conn.(utils.MultiWriter); ok {
			return mr.WriteBuffers(buffers)
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
