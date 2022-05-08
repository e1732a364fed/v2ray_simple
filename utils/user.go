package utils

import (
	"io"
	"net/url"
	"strings"
)

//User是一个唯一身份标识。
type User interface {
	GetIdentityStr() string //每个user唯一，通过比较这个string 即可 判断两个User 是否相等

	GetIdentityBytes() []byte
}

type UserHaser interface {
	HasUserByBytes(bs []byte) bool
	UserBytesLen() int
}

type UserContainer interface {
	UserHaser

	GetUserByStr(idStr string) User
	GetUserByBytes(bs []byte) User
}

// 可以控制 User 登入和登出 的接口
type UserBus interface {
	AddUser(User) error
	DelUser(User)
}

type UserConn interface {
	io.ReadWriter
	User
	GetProtocolVersion() int
}

//一种专门用于v2ray协议族(vmess/vless)的 用于标识用户的符号 , 实现 User 接口
type V2rayUser [16]byte

func (u V2rayUser) GetIdentityStr() string {
	return UUIDToStr(u)
}

func (u V2rayUser) GetIdentityBytes() []byte {
	return u[:]
}

func NewV2rayUser(s string) (V2rayUser, error) {
	uuid, err := StrToUUID(s)
	if err != nil {
		return V2rayUser{}, err
	}

	return uuid, nil
}

//used in proxy/socks5 and proxy.http
type EasyUserPassHolder struct {
	User, Password []byte
}

//	return len(ph.User) > 0 && len(ph.Password) > 0
func (ph *EasyUserPassHolder) HasUserPass() bool {
	return len(ph.User) > 0 && len(ph.Password) > 0
}

//require "user" and "pass" field
func (ph *EasyUserPassHolder) InitWithUrl(u *url.URL) {
	ph.User = []byte(u.Query().Get("user"))
	ph.User = []byte(u.Query().Get("pass"))
}

//uuid: "user:xxxx\npass:xxxx"
func (ph *EasyUserPassHolder) InitWithStr(str string) {
	strs := strings.SplitN(str, "\n", 2)
	if len(strs) != 2 {

		return

	}

	var potentialUser, potentialPass string

	ustrs := strings.SplitN(strs[0], ":", 2)
	if ustrs[0] != "user" {

		return
	}
	potentialUser = ustrs[1]

	pstrs := strings.SplitN(strs[1], ":", 2)
	if pstrs[0] != "pass" {

		return
	}
	potentialPass = pstrs[1]

	if potentialUser != "" && potentialPass != "" {
		ph.User = []byte(potentialUser)
		ph.Password = []byte(potentialPass)
	}
}
