/*Package vmess implements vmess client.

from github.com/Dreamacro/clash/tree/master/transport/vmess/

本作不支持alterid!=0 的情况. 即 仅支持 使用 aead 方式 进行认证

标准:  https://www.v2fly.org/developer/protocols/vmess.html

aead:

https://github.com/v2fly/v2fly-github-io/issues/20
*/
package vmess

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const Name = "vmess"

// Request Options
const (
	OptBasicFormat byte = 0 // 基本格式
	OptChunkStream byte = 1 // 标准格式,实际的请求数据被分割为若干个小块
)

// Security types
const (
	SecurityAES128GCM        byte = 3
	SecurityChacha20Poly1305 byte = 4
	SecurityNone             byte = 5
)

//v2ray CMD types
const (
	CmdTCP byte = 1
	CmdUDP byte = 2
)

var getkeyBs = []byte("c48619fe-8f02-49e0-b9e9-edf763e17e21")

// GetKey returns the key of AES-128-CFB encrypter
// Key：MD5(UUID + []byte('c48619fe-8f02-49e0-b9e9-edf763e17e21'))
func GetKey(uuid [16]byte) []byte {
	md5hash := md5.New()
	md5hash.Write(uuid[:])
	md5hash.Write(getkeyBs)
	return md5hash.Sum(nil)
}

// TimestampHash returns the iv of AES-128-CFB encrypter
// IV：MD5(X + X + X + X)，X = []byte(timestamp.now) (8 bytes, Big Endian)
func TimestampHash(unixSec int64) []byte {
	ts := utils.GetBytes(8)
	defer utils.PutBytes(ts)

	binary.BigEndian.PutUint64(ts, uint64(unixSec))
	md5hash := md5.New()
	md5hash.Write(ts)
	md5hash.Write(ts)
	md5hash.Write(ts)
	md5hash.Write(ts)
	return md5hash.Sum(nil)
}

//依照 vmess 协议的格式 依次读取 地址的 port, 域名/ip 信息(与vless相同)
func GetAddrFrom(buf utils.ByteReader) (addr netLayer.Addr, err error) {

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
		err = utils.ErrInvalidData
		return
	}
	addr.Port = int(port)

	var b1 byte

	b1, err = buf.ReadByte()
	if err != nil {
		return
	}

	switch b1 {
	case netLayer.AtypDomain:
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

	case netLayer.AtypIP4:
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
	case netLayer.AtypIP6:
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
		err = utils.ErrInvalidData
		return
	}

	return
}
