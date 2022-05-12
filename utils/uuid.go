package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const UUID_BytesLen = 16

func StrToUUID_slice(s string) []byte {
	bs, err := StrToUUID(s)
	if err != nil {
		return nil
	}
	return bs[:]
}

func StrToUUID(s string) (uuid [UUID_BytesLen]byte, err error) {
	if len(s) != 36 {
		return uuid, ErrInErr{ErrDesc: "invalid UUID Str", ErrDetail: ErrInvalidData, Data: s}
	}
	b := []byte(strings.Replace(s, "-", "", -1))
	if len(b) != 32 {
		return uuid, ErrInErr{ErrDesc: "invalid UUID Str", ErrDetail: ErrInvalidData, Data: s}
	}
	_, err = hex.Decode(uuid[:], b)
	return
}

func UUIDToStr(u []byte) string {
	if len(u) != UUID_BytesLen {
		return ""
	}
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

//生成完全随机的uuid
func GenerateUUID() (r [UUID_BytesLen]byte) {
	rand.Reader.Read(r[:])
	return
}
func GenerateUUIDStr() string {
	bs := GenerateUUID()
	return UUIDToStr(bs[:])
}

//生成符合v4标准的uuid
func GenerateUUID_v4() (r [UUID_BytesLen]byte) {
	rand.Reader.Read(r[:])
	r[6] = (r[6] & 0x0f) | 0x40 // Version 4
	r[8] = (r[8] & 0x3f) | 0x80 // Variant is 10，（标准要求 "8", "9", "a", or "b"，我们是第十种）
	return
}
