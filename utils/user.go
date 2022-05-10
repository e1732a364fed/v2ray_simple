package utils

import (
	"bytes"
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

type UserPassMatcher interface {
	GetUserByPass(user, pass []byte) User
}

type UserGetter interface {
	GetUserByStr(idStr string) User
	GetUserByBytes(bs []byte) User
}
type UserContainer interface {
	UserHaser

	UserGetter
}

type UserPassContainer interface {
	UserPassMatcher

	UserGetter
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

type UserConf struct {
	User string `toml:"user"`
	Pass string `toml:"pass"`
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

//used in proxy/socks5 and proxy.http. implements User, UserPassMatcher, UserPassContainer
type SingleUserWithPass struct {
	UserID, Password []byte
}

func (ph *SingleUserWithPass) GetIdentityStr() string {
	return string(ph.UserID)
}

func (ph *SingleUserWithPass) GetIdentityBytes() []byte {
	return ph.UserID
}

//	return len(ph.User) > 0 && len(ph.Password) > 0
func (ph *SingleUserWithPass) Valid() bool {
	return len(ph.UserID) > 0 && len(ph.Password) > 0
}

func (ph *SingleUserWithPass) GetUserByPass(user, pass []byte) User {
	if bytes.Equal(user, ph.UserID) && bytes.Equal(pass, ph.Password) {
		return ph
	}
	return nil
}

func (ph *SingleUserWithPass) GetUserByStr(idStr string) User {
	if idStr == string(ph.UserID) {
		return ph
	}
	return nil
}
func (ph *SingleUserWithPass) GetUserByBytes(bs []byte) User {
	if bytes.Equal(bs, ph.UserID) {
		return ph
	}
	return nil
}

//require "user" and "pass" field
func (ph *SingleUserWithPass) InitWithUrl(u *url.URL) {
	ph.UserID = []byte(u.Query().Get("user"))
	ph.UserID = []byte(u.Query().Get("pass"))
}

//uuid: "user:xxxx\npass:xxxx"
func (ph *SingleUserWithPass) InitWithStr(str string) {
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
		ph.UserID = []byte(potentialUser)
		ph.Password = []byte(potentialPass)
	}
}
