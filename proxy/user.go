package proxy

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"strings"
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

type UserServer interface {
	Server
	UserContainer
}

type UserConn interface {
	io.ReadWriter
	User
	GetProtocolVersion() int
}

//一种专门用于v2ray协议族的 User
func (u V2rayUser) GetIdentityStr() string {
	return UUIDToStr(u)
}

func (u V2rayUser) GetIdentityBytes() []byte {
	return u[:]
}

//一种专门用于v2ray协议族的 结构 (vmess/vless), 实现 User 接口
type V2rayUser [16]byte

func NewV2rayUser(s string) (*V2rayUser, error) {
	uuid, err := StrToUUID(s)
	if err != nil {
		return nil, err
	}

	return (*V2rayUser)(&uuid), nil
}

func StrToUUID(s string) (uuid [16]byte, err error) {
	b := []byte(strings.Replace(s, "-", "", -1))
	if len(b) != 32 {
		return uuid, errors.New("invalid UUID: " + s)
	}
	_, err = hex.Decode(uuid[:], b)
	return
}

func UUIDToStr(u [16]byte) string {
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], u[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], u[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], u[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], u[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], u[10:])
	return string(buf)
}

// GetKey returns the key of AES-128-CFB encrypter
// Key：MD5(UUID + []byte('c48619fe-8f02-49e0-b9e9-edf763e17e21'))
func Get_cmdKey(uuid [16]byte) []byte {
	md5hash := md5.New()
	md5hash.Write(uuid[:])
	md5hash.Write([]byte("c48619fe-8f02-49e0-b9e9-edf763e17e21"))
	return md5hash.Sum(nil)
}
