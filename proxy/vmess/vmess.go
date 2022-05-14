/*Package vmess implements vmess client.

from github.com/Dreamacro/clash/tree/master/transport/vmess/
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
	OptBasicFormat byte = 0 // 不加密传输
	OptChunkStream byte = 1 // 分块传输，每个分块使用如下Security方法加密
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

// GetKey returns the key of AES-128-CFB encrypter
// Key：MD5(UUID + []byte('c48619fe-8f02-49e0-b9e9-edf763e17e21'))
func GetKey(uuid [16]byte) []byte {
	md5hash := md5.New()
	md5hash.Write(uuid[:])
	md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21"))
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
