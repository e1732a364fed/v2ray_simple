package utils

import (
	"bytes"
	"io"
	"net/url"
	"strings"
	"sync"
)

//User是一个唯一身份标识。
type User interface {
	GetIdentityStr() string //每个user唯一，通过比较这个string 即可 判断两个User 是否相等

	GetIdentityBytes() []byte //与str类似; 对于程序来说,bytes更方便处理; 可以与str相同，也可以不同.
}

type UserWithPass interface {
	User
	GetPassword() []byte
}

//判断用户是否存在，但不提取。
type UserHaser interface {
	HasUserByBytes(bs []byte) bool
	UserBytesLen() int //用户名bytes的最小长度
}

//匹配用户名和密码，可用于auth
type UserPassMatcher interface {
	GetUserByPass(user, pass []byte) User
}

//提取一个User
type UserGetter interface {
	GetUserByStr(idStr string) User
	GetUserByBytes(bs []byte) User
}

//可判断是否存在，也可以提取
type UserContainer interface {
	UserHaser

	UserGetter
}

//可判断是否存在，也可以通过用户名、密码提取
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
	return UUIDToStr(u[:])
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
type UserPass struct {
	UserID, Password []byte
}

func NewUserPass(uc UserConf) *UserPass {
	return &UserPass{
		UserID:   []byte(uc.User),
		Password: []byte(uc.Pass),
	}
}

func (ph *UserPass) GetIdentityStr() string {
	return string(ph.UserID)
}

func (ph *UserPass) GetIdentityBytes() []byte {
	return ph.UserID
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

func (ph *UserPass) GetUserByStr(idStr string) User {
	if idStr == string(ph.UserID) {
		return ph
	}
	return nil
}
func (ph *UserPass) GetUserByBytes(bs []byte) User {
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

//implements UserBus, UserHaser, UserGetter, UserPassMatcher; 只能存储同一类型的User.
// 通过 bytes存储用户id，而不是 str。
type MultiUserMap struct {
	UserPassMap map[string]User

	Mutex sync.RWMutex

	TheUserBytesLen int

	Key_StrToBytesFunc func(string) []byte //如果这一项给出, 则内部会用 identityStr 作为key;否则会用 string(identityBytes)作为key

	Key_BytesToStrFunc func([]byte) string //必须与 Key_StrToBytesFunc 同时给出
}

func NewMultiUserMap() *MultiUserMap {
	mup := &MultiUserMap{}
	mup.UserPassMap = make(map[string]User)
	return mup
}

func (mu *MultiUserMap) SetUseUUIDStr_asKey() {
	mu.TheUserBytesLen = UUID_BytesLen
	mu.Key_BytesToStrFunc = UUIDToStr
	mu.Key_StrToBytesFunc = StrToUUID_slice
}

func (mu *MultiUserMap) AddUser(u User) error {
	mu.Mutex.Lock()
	mu.addUser(u)
	mu.Mutex.Unlock()

	return nil
}

func (mu *MultiUserMap) addUser(u User) {
	if mu.Key_StrToBytesFunc != nil {
		mu.UserPassMap[u.GetIdentityStr()] = u

	} else {
		mu.UserPassMap[string(u.GetIdentityBytes())] = u
	}
}

func (mu *MultiUserMap) DelUser(u User) error {
	mu.Mutex.Lock()

	if mu.Key_StrToBytesFunc != nil {
		delete(mu.UserPassMap, u.GetIdentityStr())

	} else {
		delete(mu.UserPassMap, string(u.GetIdentityBytes()))

	}

	mu.Mutex.Unlock()

	return nil
}

func (mu *MultiUserMap) LoadUsers(uc []UserConf) {
	mu.Mutex.Lock()
	defer mu.Mutex.Unlock()

	for _, us := range uc {
		u := NewUserPass(us)
		mu.addUser(u)
	}
}

func (mu *MultiUserMap) HasUserByBytes(bs []byte) bool {
	mu.Mutex.RLock()
	defer mu.Mutex.RUnlock()

	if mu.Key_BytesToStrFunc != nil {
		return mu.UserPassMap[mu.Key_BytesToStrFunc(bs)] != nil

	} else {
		return mu.UserPassMap[string(bs)] != nil

	}

}

func (mu *MultiUserMap) UserBytesLen() int {
	return mu.TheUserBytesLen
}

func (mu *MultiUserMap) GetUserByStr(str string) User {
	mu.Mutex.RLock()

	var u User

	if mu.Key_StrToBytesFunc != nil {

		u = mu.UserPassMap[string(mu.Key_StrToBytesFunc(str))]

	} else {
		u = mu.UserPassMap[str]

	}

	mu.Mutex.RUnlock()
	return u
}
func (mu *MultiUserMap) GetUserByBytes(bs []byte) User {
	mu.Mutex.RLock()

	var u User

	if mu.Key_BytesToStrFunc != nil {

		u = mu.UserPassMap[mu.Key_BytesToStrFunc(bs)]

	} else {
		u = mu.UserPassMap[string(bs)]

	}

	mu.Mutex.RUnlock()
	return u
}

func (mu *MultiUserMap) GetUserByPass(user, pass []byte) User {
	u := mu.GetUserByBytes(user)

	if u == nil {
		return nil
	}
	if up, ok := u.(UserWithPass); ok {
		if bytes.Equal(pass, up.GetPassword()) {
			return up
		}
		return nil
	}
	if len(pass) == 0 {
		return u
	}
	return nil

}
