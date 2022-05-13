package utils

import (
	"bytes"
	"net/url"
	"strings"
	"sync"
)

//User是一个 可确定唯一身份，且可验证该身份的 标识。
type User interface {
	IdentityStr() string //每个user唯一，通过比较这个string 即可 判断两个User 是否相等。相当于 user name

	IdentityBytes() []byte //与str类似; 对于程序来说,bytes更方便处理; 可以与str相同，也可以不同.

	AuthStr() string   //AuthStr 可以识别出该用户 并验证该User的真实性。相当于 user name + password
	AuthBytes() []byte //与 AuthStr 类似
}

type UserWithPass interface {
	User
	GetPassword() []byte
}

//判断用户是否存在并取出
type UserHaser interface {
	HasUserByBytes(bs []byte) User
	IDBytesLen() int //用户名bytes的最小长度
}

//通过验证信息 试图取出 一个User
type UserAuther interface {
	AuthUserByStr(idStr string) User
	AuthUserByBytes(bs []byte) User
	AuthBytesLen() int
}

//可判断是否存在，也可以验证
type UserContainer interface {
	UserHaser

	UserAuther
}

// 可以控制 User 登入和登出 的接口
type UserBus interface {
	AddUser(User) error
	DelUser(User)
}

type UserConf struct {
	User string `toml:"user"`
	Pass string `toml:"pass"`
}

func InitV2rayUsers(uc []UserConf) (us []User) {
	us = make([]User, len(uc))
	for i, theuc := range uc {
		var vu V2rayUser
		copy(vu[:], StrToUUID_slice(theuc.User))
		us[i] = vu
	}
	return
}

//一种专门用于v2ray协议族(vmess/vless)的 用于标识用户的符号 , 实现 User 接口
type V2rayUser [16]byte

func (u V2rayUser) IdentityStr() string {
	return UUIDToStr(u[:])
}
func (u V2rayUser) AuthStr() string {
	return u.IdentityStr()
}

func (u V2rayUser) IdentityBytes() []byte {
	return u[:]
}
func (u V2rayUser) AuthBytes() []byte {
	return u[:]
}

func NewV2rayUser(s string) (V2rayUser, error) {
	uuid, err := StrToUUID(s)
	if err != nil {
		return V2rayUser{}, err
	}

	return uuid, nil
}

//used in proxy/socks5 and proxy.http. implements User
type UserPass struct {
	UserID, Password []byte
}

func NewUserPass(uc UserConf) *UserPass {
	return &UserPass{
		UserID:   []byte(uc.User),
		Password: []byte(uc.Pass),
	}
}

func (ph *UserPass) IdentityStr() string {
	return string(ph.UserID)
}

func (ph *UserPass) IdentityBytes() []byte {
	return ph.UserID
}

func (ph *UserPass) AuthStr() string {
	return string(ph.UserID) + "\n" + string(ph.Password)
}

func (ph *UserPass) AuthBytes() []byte {
	return []byte(ph.AuthStr())
}

//	return len(ph.User) > 0 && len(ph.Password) > 0
func (ph *UserPass) Valid() bool {
	return len(ph.UserID) > 0 && len(ph.Password) > 0
}

func (ph *UserPass) GetUserByPass(user, pass []byte) User {
	if bytes.Equal(user, ph.UserID) && bytes.Equal(pass, ph.Password) {
		return ph
	}
	return nil
}

func (ph *UserPass) AuthUserByStr(idStr string) User {
	if idStr == string(ph.UserID) {
		return ph
	}
	return nil
}
func (ph *UserPass) AuthUserByBytes(bs []byte) User {
	if bytes.Equal(bs, ph.UserID) {
		return ph
	}
	return nil
}

//require "user" and "pass" field
func (ph *UserPass) InitWithUrl(u *url.URL) {
	ph.UserID = []byte(u.Query().Get("user"))
	ph.UserID = []byte(u.Query().Get("pass"))
}

//uuid: "user:xxxx\npass:xxxx"
func (ph *UserPass) InitWithStr(str string) {
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

//implements UserBus, UserHaser, UserGetter; 只能存储同一类型的User.
// 通过 bytes存储用户id，而不是 str。
type MultiUserMap struct {
	IDMap   map[string]User
	AuthMap map[string]User

	Mutex sync.RWMutex

	TheIDBytesLen, TheAuthBytesLen int

	StoreKeyAsStr bool //如果这一项给出, 则内部会用 identityStr/AuthStr 作为key;否则会用 string(identityBytes) 或 string(AuthBytes) 作为key

	IDStrToBytesFunc func(string) []byte

	IDBytesToStrFunc func([]byte) string //必须与 Key_StrToBytesFunc 同时给出

	AuthStrToBytesFunc func(string) []byte

	AuthBytesToStrFunc func([]byte) string //必须与 Key_AuthStrToBytesFunc 同时给出

}

func NewMultiUserMap() *MultiUserMap {
	mup := &MultiUserMap{
		IDMap:   make(map[string]User),
		AuthMap: make(map[string]User),
	}

	return mup
}

func (mu *MultiUserMap) SetUseUUIDStr_asKey() {
	//uuid 既是 id 又是 auth

	mu.StoreKeyAsStr = true
	mu.TheIDBytesLen = UUID_BytesLen
	mu.TheAuthBytesLen = UUID_BytesLen
	mu.IDBytesToStrFunc = UUIDToStr
	mu.AuthBytesToStrFunc = UUIDToStr
	mu.IDStrToBytesFunc = StrToUUID_slice
	mu.AuthStrToBytesFunc = StrToUUID_slice
}

func (mu *MultiUserMap) AddUser(u User) error {
	mu.Mutex.Lock()
	mu.addUser(u)
	mu.Mutex.Unlock()

	return nil
}

func (mu *MultiUserMap) addUser(u User) {
	if mu.StoreKeyAsStr {

		mu.IDMap[u.IdentityStr()] = u
		mu.AuthMap[u.AuthStr()] = u

	} else {

		mu.IDMap[string(u.IdentityBytes())] = u
		mu.AuthMap[string(u.AuthBytes())] = u
	}
}

func (mu *MultiUserMap) DelUser(u User) error {
	mu.Mutex.Lock()

	if mu.StoreKeyAsStr {
		delete(mu.IDMap, u.AuthStr())
		delete(mu.AuthMap, u.IdentityStr())

	} else {
		delete(mu.IDMap, string(u.AuthBytes()))
		delete(mu.AuthMap, string(u.IdentityBytes()))

	}

	mu.Mutex.Unlock()

	return nil
}

func (mu *MultiUserMap) LoadUsers(us []User) {
	mu.Mutex.Lock()
	defer mu.Mutex.Unlock()

	for _, u := range us {
		mu.addUser(u)
	}
}

//通过ID查找
func (mu *MultiUserMap) HasUserByStr(str string) bool {
	mu.Mutex.RLock()
	defer mu.Mutex.RUnlock()

	if !mu.StoreKeyAsStr && mu.IDStrToBytesFunc != nil {

		return mu.IDMap[string(mu.IDStrToBytesFunc(str))] != nil

	} else {
		return mu.IDMap[str] != nil

	}
}

//通过ID查找
func (mu *MultiUserMap) HasUserByBytes(bs []byte) User {
	mu.Mutex.RLock()
	defer mu.Mutex.RUnlock()

	if mu.StoreKeyAsStr && mu.IDBytesToStrFunc != nil {

		return mu.IDMap[mu.IDBytesToStrFunc(bs)]

	} else {
		return mu.IDMap[string(bs)]

	}

}

func (mu *MultiUserMap) IDBytesLen() int {
	return mu.TheIDBytesLen
}

func (mu *MultiUserMap) AuthBytesLen() int {
	return mu.TheAuthBytesLen
}

//通过Auth查找
func (mu *MultiUserMap) AuthUserByStr(str string) User {
	mu.Mutex.RLock()

	var u User

	if !mu.StoreKeyAsStr && mu.AuthStrToBytesFunc != nil {

		u = mu.AuthMap[string(mu.AuthStrToBytesFunc(str))]

	} else {
		u = mu.AuthMap[str]

	}

	mu.Mutex.RUnlock()
	return u
}

//通过Auth查找
func (mu *MultiUserMap) AuthUserByBytes(bs []byte) User {
	mu.Mutex.RLock()

	var u User

	if mu.StoreKeyAsStr && mu.AuthBytesToStrFunc != nil {

		u = mu.AuthMap[mu.AuthBytesToStrFunc(bs)]

	} else {
		u = mu.AuthMap[string(bs)]

	}

	mu.Mutex.RUnlock()
	return u
}
