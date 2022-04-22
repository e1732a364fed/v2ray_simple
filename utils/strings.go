package utils

import (
	"math/rand"
	"strings"
)

//6-11 字节的字符串
func GenerateRandomString() string {

	lenth := rand.Intn(6) + 6

	var sb strings.Builder
	for i := 0; i < lenth; i++ {
		sb.WriteByte(GenerateRandomChar())
	}
	return sb.String()
}

//ascii 97-122
func GenerateRandomChar() byte {

	return byte(rand.Intn(25+1) + 97)

}
