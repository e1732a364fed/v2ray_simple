package vless

import (
	"bytes"
	"errors"
	"io"
	"net"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//实现 net.Conn, io.ReaderFrom, utils.UserConn, utils.MultiWriter, utils.MultiReader, netLayer.Splicer
type UserTCPConn struct {
	net.Conn
	optionalReader io.Reader //在服务端 使用了缓存读取握手包头后，就产生了buffer中有剩余数据的可能性，此时就要使用MultiReader

	remainFirstBufLen int //记录 服务端 读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	uuid             [16]byte
	convertedUUIDStr string
	version          int
	isServerEnd      bool //for v0

	isntFirstPacket bool //for v0

	rr syscall.RawConn   //用于 Readbuffers
	mr utils.MultiReader //用于 Readbuffers
}

func (c *UserTCPConn) GetProtocolVersion() int {
	return c.version
}
func (c *UserTCPConn) GetIdentityStr() string {
	if c.convertedUUIDStr == "" {
		c.convertedUUIDStr = utils.UUIDToStr(c.uuid)
	}

	return c.convertedUUIDStr
}

//当前连接状态是否可以直接写入底层Conn而不经任何改动/包装
func (c *UserTCPConn) canDirectWrite() bool {
	return c.version == 1 || (c.version == 0 && !(c.isServerEnd && !c.isntFirstPacket))
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

	if !c.canDirectWrite() {
		return
	}

	if netLayer.IsBasicConn(c.Conn) {
		r = true
		conn = c.Conn

	} else if s, ok := c.Conn.(netLayer.Splicer); ok {
		r, conn = s.CanSplice()
	}

	return
}

func (c *UserTCPConn) WillReadBuffersBenifit() bool {
	return c.rr != nil || c.mr != nil
}

func (c *UserTCPConn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.canDirectWrite() {

		//底层连接可以是 ws/ tls/ 基本连接; tls 我们暂不支持 utils.MultiWriter
		// 理论上tls是可以支持的，但是要我们魔改tls库, 所以再说

		//本作的 ws.Conn 实现了 utils.MultiWriter

		if c.underlayIsBasic {

			return utils.BuffersWriteTo(buffers, c.Conn)

		} else if mr, ok := c.Conn.(utils.MultiWriter); ok {

			return mr.WriteBuffers(buffers)
		}
	}
	//发现用tls时，下面的 MergeBuffers然后一起写入的方式，能提供巨大的性能提升
	// 应该是因为, 每一次调用tls.Write 都会有一定的开销, 如果能合在一起再写入，就能避免多次写入的开销

	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := c.Write(bigbs)

	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}

//如果是udp，则是多线程不安全的，如果是tcp，则安不安全看底层的链接。
// 这里规定，如果是UDP，则 每Write一遍，都要Write一个 完整的UDP 数据包
func (c *UserTCPConn) Write(p []byte) (int, error) {

	if c.version == 0 {

		originalSupposedWrittenLenth := len(p)

		var writeBuf *bytes.Buffer

		if c.isServerEnd && !c.isntFirstPacket {
			c.isntFirstPacket = true

			writeBuf = utils.GetBuf()

			//v0 中，服务端的回复的第一个包也是要有数据头的(和客户端的handshake类似，只是第一个包有)，第一字节版本，第二字节addon长度（都是0）

			writeBuf.WriteByte(0)
			writeBuf.WriteByte(0)

		}

		if writeBuf != nil {
			writeBuf.Write(p)

			_, err := c.Conn.Write(writeBuf.Bytes())

			utils.PutBuf(writeBuf)

			if err != nil {
				return 0, err
			}

		} else {
			_, err := c.Conn.Write(p)

			if err != nil {
				return 0, err
			}
		}
		return originalSupposedWrittenLenth, nil

	} else {
		return c.Conn.Write(p)

	}
}

//专门适用于 裸奔splice的情况
func (c *UserTCPConn) ReadFrom(r io.Reader) (written int64, err error) {

	return netLayer.TryReadFrom_withSplice(c, c.Conn, r, c.canDirectWrite)
}

//如果是udp，则是多线程不安全的，如果是tcp，则安不安全看底层的链接。
// 这里规定，如果是UDP，则 每次 Read 得到的都是一个 完整的UDP 数据包，除非p给的太小……
func (c *UserTCPConn) Read(p []byte) (int, error) {

	if c.isServerEnd {
		var from io.Reader = c.Conn
		if c.optionalReader != nil {
			from = c.optionalReader
		}

		if c.remainFirstBufLen > 0 {

			n, err := from.Read(p)

			if n > 0 {
				c.remainFirstBufLen -= n
				if c.remainFirstBufLen <= 0 {
					c.optionalReader = nil
				}
			}
			return n, err

		} else {
			return c.Conn.Read(p)
		}

	} else if c.version == 0 {

		if !c.isntFirstPacket {
			//先读取响应头

			c.isntFirstPacket = true

			bs := utils.GetPacket()
			n, e := c.Conn.Read(bs)

			if e != nil {
				utils.PutPacket(bs)
				return 0, e
			}

			if n < 2 {
				utils.PutPacket(bs)
				return 0, errors.New("vless response head too short")
			}
			n = copy(p, bs[2:n])
			utils.PutPacket(bs)
			return n, nil

		} else {
			return c.Conn.Read(p)

		}

	} else {
		return c.Conn.Read(p)

	}
}

func (c *UserTCPConn) ReadBuffers() (bs [][]byte, err error) {

	if !c.isServerEnd {

		if c.version == 0 && !c.isntFirstPacket {

			c.isntFirstPacket = true

			packet := utils.GetPacket()
			var n int
			n, err = c.Read(packet)
			if err != nil {
				utils.PutPacket(packet)
				return
			}
			if n < 2 {
				utils.PutPacket(packet)
				return nil, errors.New("vless response head too short")
			}
			bs = append(bs, packet[2:n])
			return

		} else {

			return netLayer.ReadBuffersFrom(c.Conn, c.rr, c.mr)

		}

	} else {

		if c.remainFirstBufLen > 0 { //firstPayload 已经被最开始的main.go 中的 Read读掉了，所以 在调用 ReadBuffers 时 c.remainFirstBufLen 一般为 0, 所以一般不会调用这里

			return netLayer.ReadBuffersFrom(c.optionalReader, nil, nil)

		} else {

			return netLayer.ReadBuffersFrom(c.Conn, c.rr, c.mr)

		}

	}

}
