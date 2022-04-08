package trojan

import (
	"bufio"
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

//trojan比较简洁，这个 UserTCPConn 只是用于读取握手读取时读到的剩余的缓存
type UserTCPConn struct {
	net.Conn
	optionalReader io.Reader //在使用了缓存读取握手包头后，就产生了buffer中有剩余数据的可能性，此时就要使用MultiReader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	hash        string
	isServerEnd bool //for v0

	bufr            *bufio.Reader //for udp
	isntFirstPacket bool          //for v0

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
	if c.remainFirstBufLen > 0 {
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
