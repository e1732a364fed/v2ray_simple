package utils

import (
	"math/rand"
	"strings"
	"time"
)

func init() {
	//保证我们随机种子每次都不一样, 这样可以保证go test中使用随机端口时, 不同的test会使用不同的端口, 防止端口冲突
	// 因为我们所有的包应该都引用了 utils包, 所以可以保证这一点.
	rand.Seed(time.Now().Unix())
}

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
