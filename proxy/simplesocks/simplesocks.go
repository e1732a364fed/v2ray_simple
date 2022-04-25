//Package simplesocks implements SimpleSocks protocol.
// See https://p4gefau1t.github.io/trojan-go/developer/simplesocks/
package simplesocks

import (
	"errors"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	// trojan-go 的代码里，这里依然叫做connect和associate，而实际上，因为通道已经建立，所以实际含义已经不同
	// 这些命令只用于指示承载数据的传输层协议，所以我们这里重命名一下。
	CmdTCP = 0x01
	CmdUDP = 0x03

	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4

	Name = "simplesocks"
)

var (
	crlf = []byte{0x0d, 0x0a}
)

//依照trojan协议的格式读取 地址的域名、ip、port信息
func GetAddrFrom(buf utils.ByteReader) (addr netLayer.Addr, err error) {
	var b1 byte

	b1, err = buf.ReadByte()
	if err != nil {
		return
	}

	switch b1 {
	case ATypDomain:
		var b2 byte
		b2, err = buf.ReadByte()
		if err != nil {
			return
		}

		if b2 == 0 {
			err = errors.New("got ATypDomain but domain lenth is marked to be 0")
			return
		}

		bs := utils.GetBytes(int(b2))
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}

		if n != int(b2) {
			err = utils.ErrShortRead
			return
		}
		addr.Name = string(bs[:n])

	case ATypIP4:
		bs := make([]byte, 4)
		var n int
		n, err = buf.Read(bs)

		if err != nil {
			return
		}
		if n != 4 {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	case ATypIP6:
		bs := make([]byte, net.IPv6len)
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}
		if n != 4 {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	default:
		err = utils.ErrInErr{ErrDesc: "simplesocks GetAddrFrom err", ErrDetail: utils.ErrInvalidData, Data: b1}
		return
	}

	pb1, err := buf.ReadByte()
	if err != nil {
		return
	}

	pb2, err := buf.ReadByte()
	if err != nil {
		return
	}

	port := uint16(pb1)<<8 + uint16(pb2)
	if port == 0 {
		err = utils.ErrInErr{ErrDesc: "simplesocks port is zero, which is bad", ErrDetail: utils.ErrInvalidData}

		return
	}
	addr.Port = int(port)

	return
}
