package vless

import (
	"bufio"
	"bytes"
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

	// udpUnreadPart 不为空，则表示上一次读取没读完整个包（给Read传入的buf太小），接着读
	udpUnreadPart []byte //for v0

	bufr *bufio.Reader //for v0
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

	if uc.version == 0 && uc.isUDP {
		l := int16(len(p))
		buf := &bytes.Buffer{}
		buf.WriteByte(byte(l >> 8))
		buf.WriteByte(byte(l << 8 >> 8))
		buf.Write(p)

		_, err := uc.Conn.Write(buf.Bytes()) //直接return这个是错的，因为写入长度只能小于等于len(p)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	} else {
		return uc.Conn.Write(p)

	}
}
func (uc *UserConn) Read(p []byte) (int, error) {

	if uc.version == 0 && uc.isUDP {

		if len(uc.udpUnreadPart) > 0 {
			copiedN := copy(p, uc.udpUnreadPart)
			if copiedN < len(uc.udpUnreadPart) {
				uc.udpUnreadPart = uc.udpUnreadPart[copiedN:]
			} else {
				uc.udpUnreadPart = nil
			}
			return copiedN, nil
		}

		if uc.bufr == nil {
			uc.bufr = bufio.NewReader(uc.Conn)
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
	} else {
		return uc.Conn.Read(p)

	}
}
