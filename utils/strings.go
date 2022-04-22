package utils

import (
	"math/rand"
	"strings"

	"github.com/tjarratt/babble"
	"go.uber.org/zap"
)

func GetRandomWord() (result string) {
	//babbler包 在 系统中 没有 /usr/share/dict/words 且不是windows 时，会panic
	defer func() {

		if r := recover(); r != nil {
			if ce := CanLogErr("getRandomWord babble panic"); ce != nil {
				ce.Write(zap.Any("err:", r))
			}

			result = GenerateRandomString()
		}
	}()
	babbler := babble.NewBabbler()
	babbler.Count = 1
	result = babbler.Babble()

	return
}

//6-12 字节的字符串
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
