package vless

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//实现 net.Conn, io.ReaderFrom, utils.MultiWriter, netLayer.Splicer
type UserTCPConn struct {
	net.Conn
	optionalReader io.Reader //在使用了缓存读取握手包头后，就产生了buffer中有剩余数据的可能性，此时就要使用MultiReader

	remainFirstBufLen int //记录读取握手包头时读到的buf的长度. 如果我们读超过了这个部分的话,实际上我们就可以不再使用 optionalReader 读取, 而是直接从Conn读取

	underlayIsBasic bool

	uuid             [16]byte
	convertedUUIDStr string
	version          int
	isServerEnd      bool //for v0

	// udpUnreadPart 不为空，则表示上一次读取没读完整个包（给Read传入的buf太小），须接着读
	udpUnreadPart []byte //for udp

	bufr            *bufio.Reader //for udp
	isntFirstPacket bool          //for v0

	hasAdvancedLayer bool //for v1, 在用ws或grpc时，这个开关保持打开
}

func (uc *UserTCPConn) GetProtocolVersion() int {
	return uc.version
}
func (uc *UserTCPConn) GetIdentityStr() string {
	if uc.convertedUUIDStr == "" {
		uc.convertedUUIDStr = utils.UUIDToStr(uc.uuid)
	}

	return uc.convertedUUIDStr
}

//当前连接状态是否可以直接写入底层Conn而不经任何改动/包装
func (c *UserTCPConn) canDirectWrite() bool {
	return c.version == 1 || c.version == 0 && !(c.isServerEnd && !c.isntFirstPacket)
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

func (c *UserTCPConn) WriteBuffers(buffers [][]byte) (int64, error) {

	if c.canDirectWrite() {

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
	// 应该是因为, 每一次调用tls.Write 都会有一定的开销, 如果能合在一起再写入，就能避免多次写入的开销

	bigbs, dup := utils.MergeBuffers(buffers)
	n, e := c.Write(bigbs)
	if dup {
		utils.PutPacket(bigbs)
	}
	return int64(n), e

}

//专门适用于 裸奔splice的情况
func (uc *UserTCPConn) ReadFrom(r io.Reader) (written int64, err error) {

	return netLayer.TryReadFrom_withSplice(uc, uc.Conn, r, uc.canDirectWrite)
}

//如果是udp，则是多线程不安全的，如果是tcp，则安不安全看底层的链接。
// 这里规定，如果是UDP，则 每Write一遍，都要Write一个 完整的UDP 数据包
func (uc *UserTCPConn) Write(p []byte) (int, error) {

	if uc.version == 0 {

		originalSupposedWrittenLenth := len(p)

		var writeBuf *bytes.Buffer

		if uc.isServerEnd && !uc.isntFirstPacket {
			uc.isntFirstPacket = true

			writeBuf = utils.GetBuf()

			//v0 中，服务端的回复的第一个包也是要有数据头的(和客户端的handshake类似，只是第一个包有)，第一字节版本，第二字节addon长度（都是0）

			writeBuf.WriteByte(0)
			writeBuf.WriteByte(0)

		}

		if writeBuf != nil {
			writeBuf.Write(p)

			_, err := uc.Conn.Write(writeBuf.Bytes()) //“直接return这个的长度” 是错的，因为写入长度只能小于等于len(p)

			utils.PutBuf(writeBuf)

			if err != nil {
				return 0, err
			}
			return originalSupposedWrittenLenth, nil

		} else {
			_, err := uc.Conn.Write(p) //“直接return这个的长度” 是错的，因为写入长度只能小于等于len(p)

			if err != nil {
				return 0, err
			}
			return originalSupposedWrittenLenth, nil
		}

	} else {
		/*
			if uc.isUDP && !uc.hasAdvancedLayer {

				// 这里暂时认为包裹它的连接是 tcp或者tls，而不是udp，如果udp的话，就不需要考虑粘包问题了，比如socks5的实现
				// 我们目前认为只有tls是最防墙的，而且 魔改tls是有毒的，所以反推过来，这里udp就必须加长度头。

				// 目前是这个样子。之后verysimple实现了websocket和grpc后，会添加判断，如果连接是websocket或者grpc连接，则不再加长度头

				// tls和tcp都是基于流的，可以分开写两次，不需要buf存在；如果连接是websocket或者grpc的话，直接传输。

				l := int16(len(p))
				var lenBytes []byte

				if l <= 255 {
					lenBytes = []byte{0, byte(l)}
				}

				lenBytes = []byte{byte(l >> 8), byte(l << 8 >> 8)}

				_, err := uc.Conn.Write(lenBytes)
				if err != nil {
					return 0, err
				}

				return uc.Conn.Write(p)

			}
		*/
		return uc.Conn.Write(p)

	}
}

//如果是udp，则是多线程不安全的，如果是tcp，则安不安全看底层的链接。
// 这里规定，如果是UDP，则 每次 Read 得到的都是一个 完整的UDP 数据包，除非p给的太小……
func (uc *UserTCPConn) Read(p []byte) (int, error) {

	var from io.Reader = uc.Conn
	if uc.optionalReader != nil {
		from = uc.optionalReader
	}

	if uc.version == 0 {

		if !uc.isServerEnd && !uc.isntFirstPacket {
			//先读取响应头

			uc.isntFirstPacket = true

			bs := utils.GetPacket()
			n, e := from.Read(bs)

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

		}

		return from.Read(p)

	} else {
		/*
			if uc.isUDP && !uc.hasAdvancedLayer {

				if len(uc.udpUnreadPart) > 0 {
					copiedN := copy(p, uc.udpUnreadPart)
					if copiedN < len(uc.udpUnreadPart) {
						uc.udpUnreadPart = uc.udpUnreadPart[copiedN:]
					} else {
						uc.udpUnreadPart = nil
					}
					return copiedN, nil
				}

				return uc.readudp_withLenthHead(p)
			}*/
		return from.Read(p)

	}
}
