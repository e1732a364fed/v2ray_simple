package vless

import (
	"bufio"
	"bytes"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	flag_orig       = 0
	flag_new_source = 1
)

type UDPConn struct {
	net.Conn

	utils.V2rayUser //在 Server握手成功后会设置这一项.

	optionalReader io.Reader

	remainFirstBufLen int

	version byte

	udp_multi   bool
	isClientEnd bool

	bufr *bufio.Reader

	notFirst bool //for v0
	raddr    netLayer.Addr

	handshakeBuf *bytes.Buffer

	fullcone bool
}

func (u *UDPConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return u.Close()
}
func (u *UDPConn) Fullcone() bool {
	return u.fullcone && u.version != 0
}

func (u *UDPConn) GetProtocolVersion() int {
	return int(u.version)
}

func (u *UDPConn) WriteMsgTo(p []byte, raddr netLayer.Addr) error {

	var writeBuf *bytes.Buffer
	if u.handshakeBuf != nil {
		writeBuf = u.handshakeBuf
		u.handshakeBuf = nil
	} else {
		writeBuf = utils.GetBuf()
	}
	defer utils.PutBuf(writeBuf)

	//v0设计有问题，不支持fullcone，无视raddr，始终向最开始的raddr发送。
	if u.version == 0 {

		if !u.isClientEnd && !u.notFirst {
			u.notFirst = true

			//v0 中，服务端的回复的第一个包也是要有数据头的(和客户端的handshake类似，只是第一个包有)，第一字节版本，第二字节addon长度（都是0）

			writeBuf.WriteByte(0)
			writeBuf.WriteByte(0)

		}

		return u.writeDataTo(writeBuf, p)

	} else { // v1

		if u.udp_multi {
			if u.isClientEnd {
				//如果是客户端，则不存在raddr与初始地址不同的情况，因为这种情况已经在 netLayer.RelayUDP_separate 里过滤掉了

				return u.writeDataTo(writeBuf, p)

			} else {
				//判断raddr是否与 u.raddr相同, 如果不相同, 则要传输umfurs信息
				// umfurs信息将会提示客户端 下一次发送到此地址时，拨号一个新的 udp信道.

				if u.raddr.GetHashable() == raddr.GetHashable() {
					writeBuf.WriteByte(flag_orig)
					return u.writeDataTo(writeBuf, p)

				} else {
					writeBuf.WriteByte(flag_new_source)
					WriteAddrTo(writeBuf, raddr)
					return u.writeDataTo(writeBuf, p)

				}
			}
		} else {
			WriteAddrTo(writeBuf, raddr)
			return u.writeDataTo(writeBuf, p)
		}

	}
	//return nil

}

func (uc *UDPConn) readData_with_len() ([]byte, error) {

	b1, err := uc.bufr.ReadByte()
	if err != nil {
		return nil, err
	}

	b2, err := uc.bufr.ReadByte()
	if err != nil {
		return nil, err
	}

	l := int(uint16(b1)<<8 + uint16(b2))

	bs := utils.GetBytes(l)
	n, err := io.ReadFull(uc.bufr, bs)

	if err != nil {
		return nil, err
	}
	return bs[:n], nil

}

func (u *UDPConn) writeDataTo(writeBuf *bytes.Buffer, p []byte) error {
	writeBuf.WriteByte(byte(len(p) >> 8))
	writeBuf.WriteByte(byte(len(p) << 8 >> 8))
	_, err := u.Conn.Write(writeBuf.Bytes())

	if err != nil {
		return err
	} else {
		_, err := u.Conn.Write(p)
		return err
	}

}

func (u *UDPConn) ReadMsg() ([]byte, netLayer.Addr, error) {
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
		bs, err := u.readData_with_len()
		return bs, u.raddr, err
	} else {

		if u.bufr == nil {
			u.bufr = bufio.NewReader(from)
		}

		if u.udp_multi {
			if u.isClientEnd {
				//判断是否是 umfurs信息
				// umfurs信息将会提示客户端 下一次发送到此地址时，拨号一个新的 udp信道.
				b1, err := u.bufr.ReadByte()
				if err != nil {
					return nil, netLayer.Addr{}, err
				}
				switch b1 {
				default:
					return nil, netLayer.Addr{}, utils.ErrInErr{ErrDesc: "Vless udp_multi client read first byte unexpected", ErrDetail: utils.ErrInvalidData, Data: b1}
				case flag_orig:
					bs, err := u.readData_with_len()
					return bs, u.raddr, err
				case flag_new_source:
					raddr, err := netLayer.V2rayGetAddrFrom(u.bufr)
					if err != nil {
						return nil, raddr, err
					}
					bs, err := u.readData_with_len()
					return bs, raddr, err
				}

			} else {
				bs, err := u.readData_with_len()
				return bs, u.raddr, err
			}
		} else {
			raddr, err := netLayer.V2rayGetAddrFrom(u.bufr)
			if err != nil {
				return nil, raddr, err
			}
			bs, err := u.readData_with_len()

			return bs, raddr, err
		}
	}
}
