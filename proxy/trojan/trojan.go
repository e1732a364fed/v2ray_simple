// package trojan implements trojan protocol for proxy.Client and proxy.Server.
//
// See https://trojan-gfw.github.io/trojan/protocol .
package trojan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const Name = "trojan"

const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
)
const (
	CmdConnect      = 0x01
	CmdUDPAssociate = 0x03
	CmdMux          = 0x7f //trojan-gfw 那个文档里并没有提及Mux, 这个定义作者似乎没有在任何文档中提及，所以这个是在trojan-go的源代码文件中找到的。
)

const (
	passBytesLen = sha256.Size224
	passStrLen   = passBytesLen * 2
)

var (
	crlf = []byte{0x0d, 0x0a}
)

// 即trojan 的 任意长度密码 的 定长二进制表示。
func SHA224(password string) (r [passBytesLen]byte) {
	hash := sha256.New224()
	hash.Write([]byte(password))
	copy(r[:], hash.Sum(nil))
	return
}

// 56字节数据转28字节字符串
func PassStrToBytes(str string) []byte {
	bs, err := hex.DecodeString(str)
	if err != nil {
		return nil
	}

	return bs
}

// 28字节数据转56字节字符串
func PassBytesToStr(bs []byte) string {
	return hex.EncodeToString(bs)
}

type User struct {
	hexStr   string
	hexBs    []byte
	plainStr string
}

func NewUserByPlainTextPassword(plainPass string) User {
	bs := SHA224(plainPass)
	return User{
		hexStr:   SHA224_hexString(plainPass),
		hexBs:    bs[:],
		plainStr: plainPass,
	}
}

// 明文密码
func (u User) IdentityStr() string {
	return u.plainStr
}

// 明文密码
func (u User) IdentityBytes() []byte {
	return []byte(u.plainStr)
}

// 56字节hex
func (u User) AuthStr() string {
	return u.hexStr
}

// 28字节纯二进制
func (u User) AuthBytes() []byte {
	return u.hexBs
}

func InitUsers(uc []utils.UserConf) (us []utils.User) {
	us = make([]utils.User, len(uc))
	for i, theuc := range uc {

		us[i] = NewUserByPlainTextPassword(theuc.User)
	}
	return
}

// trojan协议 的前56字节 是 sha224的28字节 每字节 转义成 base 16, with lower-case letters for a-f 的 两个字符。
// 实际上trojan协议文档写的不严谨，它只说了用hex，没说用大写还是小写。我看它代码实现用的是小写。
func SHA224_hexStringBytes(password string) []byte {
	hash := sha256.New224()
	hash.Write([]byte(password))
	bs := hash.Sum(nil)
	r := make([]byte, passStrLen)
	hex.Encode(r, bs) //hex包Encode 使用小写字符，（但decode却是同时支持大写和小写的，怪）
	return r
}

func SHA224_hexString(password string) string {
	hash := sha256.New224()
	hash.Write([]byte(password))
	bs := hash.Sum(nil)
	return PassBytesToStr(bs)
}

// 依照trojan协议的格式读取 地址的域名、ip、port信息
func GetAddrFrom(buf utils.ByteReader, ismux bool) (addr netLayer.Addr, err error) {
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
		if n != net.IPv6len {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	default:
		err = utils.ErrInErr{ErrDesc: "trojan GetAddrFrom err", ErrDetail: utils.ErrInvalidData, Data: b1}
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
		if !ismux { //trojan-go 的实现中，第一次发起mux时，port会设成0, 域名被设为 MUX_CONN

			err = utils.ErrInErr{ErrDesc: "trojan GetAddrFrom, port is zero, which is bad", ErrDetail: utils.ErrInvalidData}
			return
		}

	}
	addr.Port = int(port)

	return
}

// https://p4gefau1t.github.io/trojan-go/developer/url/
func GenerateOfficialDraftShareURL(dialconf *proxy.DialConf) string {

	var u url.URL

	u.Scheme = Name
	u.User = url.User(dialconf.Uuid)
	if dialconf.IP != "" {
		u.Host = dialconf.IP + ":" + strconv.Itoa(dialconf.Port)
	} else {
		u.Host = dialconf.Host + ":" + strconv.Itoa(dialconf.Port)

	}
	q := u.Query()
	if dialconf.TLS {
		q.Add("security", "tls")
		if dialconf.Host != "" {
			q.Add("sni", dialconf.Host)

		}

	}
	if dialconf.AdvancedLayer != "" {
		q.Add("type", dialconf.AdvancedLayer)

		switch dialconf.AdvancedLayer {
		case "ws":
			if dialconf.Path != "" {
				q.Add("path", dialconf.Path)
			}
			if dialconf.Host != "" {
				q.Add("host", dialconf.Host)

			}
		case "grpc":

			//该草案并没有提及grpc, 所以实际上不完美。本作trojan也是可以支持grpc、quic的
			//我们参照vless的url提案进行配置 serviceName项。

			if dialconf.Path != "" {
				q.Add("serviceName", dialconf.Path)
			}

		}
	}

	u.RawQuery = q.Encode()
	if dialconf.Tag != "" {
		u.Fragment = dialconf.Tag

	}
	return u.String()
}
