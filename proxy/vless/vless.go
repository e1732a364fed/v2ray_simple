package vless

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/common"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

const Name = "vless"

const (
	Cmd_CRUMFURS byte = 4 // start from vless v1

	CRUMFURS_ESTABLISHED byte = 20

	CRUMFURS_Established_Str = "CRUMFURS_Established"
)

type UserConn struct {
	net.Conn
	uuid         [16]byte
	convertedStr string
	version      int
	isUDP        bool
	isServerEnd  bool //for v0

	// udpUnreadPart 不为空，则表示上一次读取没读完整个包（给Read传入的buf太小），接着读
	udpUnreadPart []byte //for v0

	bufr            *bufio.Reader //for v0
	isntFirstPacket bool          //for v0
}

func (uc *UserConn) GetProtocolVersion() int {
	return uc.version
}
func (uc *UserConn) GetIdentityStr() string {
	if uc.convertedStr == "" {
		uc.convertedStr = proxy.UUIDToStr(uc.uuid)
	}

	return uc.convertedStr
}

func (uc *UserConn) Write(p []byte) (int, error) {

	if uc.version == 0 {

		originalSupposedWrittenLenth := len(p)

		var writeBuf *bytes.Buffer

		if uc.isServerEnd && !uc.isntFirstPacket {
			uc.isntFirstPacket = true

			writeBuf = &bytes.Buffer{}

			//v0 中，服务端的回复的第一个包也是要有数据头的(和客户端的handshake类似，只是第一个包有)，第一字节版本，第二字节addon长度（都是0）

			writeBuf.WriteByte(0)
			writeBuf.WriteByte(0)

		}

		if !uc.isUDP {

			if writeBuf != nil {
				writeBuf.Write(p)

				_, err := uc.Conn.Write(writeBuf.Bytes()) //“直接return这个的长度” 是错的，因为写入长度只能小于等于len(p)

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
			l := int16(len(p))
			if writeBuf == nil {
				writeBuf = &bytes.Buffer{}
			}

			writeBuf.WriteByte(byte(l >> 8))
			writeBuf.WriteByte(byte(l << 8 >> 8))
			writeBuf.Write(p)

			_, err := uc.Conn.Write(writeBuf.Bytes()) //“直接return这个的长度” 是错的，因为写入长度只能小于等于len(p)
			if err != nil {
				return 0, err
			}
			return originalSupposedWrittenLenth, nil
		}

	} else {
		return uc.Conn.Write(p)

	}
}
func (uc *UserConn) Read(p []byte) (int, error) {

	if uc.version == 0 {
		if uc.bufr == nil {
			uc.bufr = bufio.NewReader(uc.Conn)
		}
		if !uc.isServerEnd && !uc.isntFirstPacket {
			bs := common.GetBytes(common.StandardBytesLength)
			n, e := uc.Conn.Read(bs)

			uc.isntFirstPacket = true
			if e != nil {
				return 0, e
			}

			if n < 2 {
				return 0, errors.New("vless response head too short")
			}
			n = copy(p, bs[2:n])
			return n, nil
		}

		if !uc.isUDP {
			return uc.Conn.Read(p)
		} else {

			if len(uc.udpUnreadPart) > 0 {
				copiedN := copy(p, uc.udpUnreadPart)
				if copiedN < len(uc.udpUnreadPart) {
					uc.udpUnreadPart = uc.udpUnreadPart[copiedN:]
				} else {
					uc.udpUnreadPart = nil
				}
				return copiedN, nil
			}

			b1, err := uc.bufr.ReadByte()
			if err != nil {
				return 0, err
			}
			b2, err := uc.bufr.ReadByte()
			if err != nil {
				return 0, err
			}

			l := int(int16(b1)<<8 + int16(b2))
			bs := common.GetBytes(l)
			n, err := io.ReadFull(uc.bufr, bs)
			if err != nil {
				return 0, err
			}

			copiedN := copy(p, bs)
			if copiedN < n { //p is short
				uc.udpUnreadPart = bs[copiedN:]
			}

			return copiedN, nil
		}

	} else {
		return uc.Conn.Read(p)

	}
}
