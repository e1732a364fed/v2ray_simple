package proxy

import (
	"io"

	"github.com/hahahrfool/v2ray_simple/utils"
)

//User是一个唯一身份标识。
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

// 可以控制 User 登入和登出 的接口, 就像一辆公交车一样，或者一座航站楼。Bus也可以理解为总线。
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

func NewV2rayUser(s string) (V2rayUser, error) {
	uuid, err := utils.StrToUUID(s)
	if err != nil {
		return V2rayUser{}, err
	}

	return uuid, nil
}
