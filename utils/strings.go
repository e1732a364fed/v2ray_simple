package utils

import (
	"bytes"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

func init() {
	//保证我们随机种子每次都不一样, 这样可以保证go test中使用随机端口时, 不同的test会使用不同的端口, 防止端口冲突
	// 因为我们所有的包应该都引用了 utils包, 所以可以保证这一点.
	rand.Seed(time.Now().Unix())
}

// 6-11 字节的字符串
func GenerateRandomString() string {

	lenth := rand.Intn(6) + 6

	var sb strings.Builder
	for i := 0; i < lenth; i++ {
		sb.WriteByte(GenerateRandomChar())
	}
	return sb.String()
}

// ascii 97-122
func GenerateRandomChar() byte {

	return byte(rand.Intn(25+1) + 97)

}

// 本来可以直接用 fmt.Print, 但是那个Print多了一次到any的装箱，
// 而且准备步骤太多, 所以如果只
// 打印一个字符串的话，不妨直接调用 os.Stdout.WriteString(str)。
func PrintStr(str string) {
	os.Stdout.WriteString(str)
}

// https://stackoverflow.com/questions/37290693/how-to-remove-redundant-spaces-whitespace-from-a-string-in-golang
func StandardizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// 从any生成toml字符串，
// 移除 = "", = 0 和 = false 的项
func GetPurgedTomlStr(v any) (string, error) {
	buf := GetBuf()
	defer PutBuf(buf)
	if err := toml.NewEncoder(buf).Encode(v); err != nil {
		return "", err
	}
	lines := strings.Split(buf.String(), "\n")
	var sb strings.Builder

	for _, l := range lines {
		if !strings.HasSuffix(l, ` = ""`) && !strings.HasSuffix(l, ` = false`) && !strings.HasSuffix(l, ` = 0`) {

			sb.WriteString(l)
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil

}

// mimic GetPurgedTomlStr
func GetPurgedTomlBytes(v any) ([]byte, error) {
	buf := GetBuf()
	defer PutBuf(buf)
	if err := toml.NewEncoder(buf).Encode(v); err != nil {
		return nil, err
	}
	lines := bytes.Split(buf.Bytes(), []byte{'\n'})
	var sb bytes.Buffer

	for _, l := range lines {
		if !bytes.HasSuffix(l, []byte(` = ""`)) && !bytes.HasSuffix(l, []byte(` = false`)) && !bytes.HasSuffix(l, []byte(` = 0`)) {

			sb.Write(l)
			sb.WriteByte('\n')
		}
	}
	return sb.Bytes(), nil

}
