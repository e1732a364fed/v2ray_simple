/*Package vmess implements vmess client.

from github.com/Dreamacro/clash/tree/master/transport/vmess/

本作不支持alterid!=0 的情况. 即 仅支持 使用 aead 方式 进行认证. 即不支持 "MD5 认证信息"

标准:  https://www.v2fly.org/developer/protocols/vmess.html

aead:

https://github.com/v2fly/v2fly-github-io/issues/20


Implementation Details

vmess 协议是一个很老旧的协议，有很多向前兼容的代码，很多地方都已经废弃了. 我们这里只支持最新的aead.

我们所实现的vmess 服务端 力求简单、最新，不求兼容所有老旧客户端。

Share URL

v2fly只有一个草案
https://github.com/v2fly/v2fly-github-io/issues/26

似乎v2fly社区对于这个URL标准的制定并不注重，而且看起来这个草案也不太美观

而xray社区的则美观得多，见 https://github.com/XTLS/Xray-core/discussions/716

*/
package vmess

import (
	"crypto/md5"
	"encoding/binary"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

const Name = "vmess"

// Request Options
const (
	OptBasicFormat byte = 0 // 基本格式
	OptChunkStream byte = 1 // 标准格式,实际的请求数据被分割为若干个小块

	OptChunkMasking byte = 4

	OptGlobalPadding byte = 0x08

	//OptAuthenticatedLength byte = 0x10
)

// Security types
const (
	SecurityAES128GCM        byte = 3
	SecurityChacha20Poly1305 byte = 4
	SecurityNone             byte = 5
)

//v2ray CMD types
const (
	CmdTCP                    byte = 1
	CmdUDP                    byte = 2
	cmd_muxcool_unimplemented byte = 3
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
