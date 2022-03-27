package proxy

import (
	"io"

	"github.com/hahahrfool/v2ray_simple/utils"
)

type User interface {
	GetIdentityStr() string //每个user唯一，通过比较这个string 即可 判断两个User 是否相等

	GetIdentityBytes() []byte
}

type UserClient interface {
	Client
	GetUser() User
}

type UserContainer interface {
	GetUserByStr(idStr string) User
	GetUserByBytes(bs []byte) User

	//tlsLayer.UserHaser
	HasUserByBytes(bs []byte) bool
	UserBytesLen() int
}

// 可以控制 User 登入和登出 的接口, 就像一辆公交车一样，或者一座航站楼
type UserBus interface {
	AddUser(User) error
	DelUser(User)
}

type UserServer interface {
	Server
	UserContainer
}

type UserConn interface {
	io.ReadWriter
	User
	GetProtocolVersion() int
}

//一种专门用于v2ray协议族(vmess/vless)的 用于标识用户的符号 , 实现 User 接口
type V2rayUser [16]byte

func (u V2rayUser) GetIdentityStr() string {
	return utils.UUIDToStr(u)
}

func (u V2rayUser) GetIdentityBytes() []byte {
	return u[:]
}

func NewV2rayUser(s string) (*V2rayUser, error) {
	uuid, err := utils.StrToUUID(s)
	if err != nil {
		return nil, err
	}

	return (*V2rayUser)(&uuid), nil
}

/*
//vmess legacy代码，先放这里，什么时候想实现vmess了再说
// GetKey returns the key of AES-128-CFB encrypter
// Key：MD5(UUID + []byte('c48619fe-8f02-49e0-b9e9-edf763e17e21'))
func Get_cmdKey(uuid [16]byte) []byte {
	md5hash := md5.New()
	md5hash.Write(uuid[:])
	md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21"))
	return md5hash.Sum(nil)
}
*/
