package vless

import (
	"bufio"
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

type UDPConn struct {
	net.Conn
	optionalReader io.Reader

	remainFirstBufLen int

	version     int
	isClientEnd bool

	bufr *bufio.Reader

	notFirst bool //for v0
	raddr    netLayer.Addr
}

func (u *UDPConn) WriteTo(p []byte, raddr netLayer.Addr) error {

	//v0很垃圾，不支持fullcone，无视raddr，始终向最开始的raddr发送。
	if u.version == 0 {
		writeBuf := utils.GetBuf()

		if !u.isClientEnd && !u.notFirst {
			u.notFirst = true

			//v0 中，服务端的回复的第一个包也是要有数据头的(和客户端的handshake类似，只是第一个包有)，第一字节版本，第二字节addon长度（都是0）

			writeBuf.WriteByte(0)
			writeBuf.WriteByte(0)

		}

		l := int16(len(p))

		writeBuf.WriteByte(byte(l >> 8))
		writeBuf.WriteByte(byte(l << 8 >> 8))

		writeBuf.Write(p)

		_, err := u.Conn.Write(writeBuf.Bytes()) //“直接return这个的长度” 是错的，因为写入长度只能小于等于len(p)

		utils.PutBuf(writeBuf)

		if err != nil {
			return err
		}
		return nil

	} else {

	}
	return nil

}

//从 客户端读取 udp请求
func (u *UDPConn) ReadFrom() ([]byte, netLayer.Addr, error) {

	var from io.Reader = u.Conn
	if u.optionalReader != nil {
		from = u.optionalReader
	}

	if u.version == 0 {

		if u.isClientEnd {
			if !u.notFirst {
				u.notFirst = true
				u.bufr = bufio.NewReader(from)

				_, err := u.bufr.ReadByte() //version byte
				if err != nil {
					return nil, netLayer.Addr{}, err
				}

				_, err = u.bufr.ReadByte() //addon len byte
				if err != nil {
					return nil, netLayer.Addr{}, err
				}

			}

		} else {
			if u.bufr == nil {
				u.bufr = bufio.NewReader(from)

			}
		}
		bs, err := u.read_with_v0_Head()
		return bs, u.raddr, err
	} else {

	}
	return nil, netLayer.Addr{}, nil
}

func (uc *UDPConn) read_with_v0_Head() ([]byte, error) {

	b1, err := uc.bufr.ReadByte()
	if err != nil {
		return nil, err
	}

	b2, err := uc.bufr.ReadByte()
	if err != nil {
		return nil, err
	}

	l := int(int16(b1)<<8 + int16(b2))

	bs := utils.GetBytes(l)
	n, err := io.ReadFull(uc.bufr, bs)

	if err != nil {
		return nil, err
	}
	return bs[:n], nil

}
