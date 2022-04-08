//package trojan implements proxy.Client and proxy.Server with trojan protocol for.
//
//See https://trojan-gfw.github.io/trojan/protocol .
package trojan

import (
	"crypto/sha256"
	"fmt"
)

const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
	name       = "trojan"
)
const (
	CmdConnect      = 0x01
	CmdUDPAssociate = 0x03
)

var (
	crlf = []byte{0x0d, 0x0a}
)

//trojan 的前56字节 是 sha224的28字节 每字节 转义成 ascii的 表示16进制的 两个字符
func SHA224_hexStringBytes(password string) []byte {
	hash := sha256.New224()
	hash.Write([]byte(password))
	val := hash.Sum(nil)
	str := ""
	for _, v := range val {
		str += fmt.Sprintf("%02x", v)
	}
	return []byte(str)
}
