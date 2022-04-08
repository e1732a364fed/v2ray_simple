// Package utils provides utilities that is used in all codes in verysimple
package utils

import (
	"errors"
	"flag"
	"math/rand"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

func init() {
	//保证我们随机种子每次都不一样, 这样可以保证go test中使用随机端口时, 不同的test会使用不同的端口, 防止端口冲突
	// 因为我们所有的包应该都引用了 utils包, 所以可以保证这一点.
	rand.Seed(time.Now().Unix())
}

func IsFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

//移除 = "" 和 = false 的项
func GetPurgedTomlStr(v any) (string, error) {
	buf := GetBuf()
	defer PutBuf(buf)
	if err := toml.NewEncoder(buf).Encode(v); err != nil {
		return "", err
	}
	lines := strings.Split(buf.String(), "\n")
	var sb strings.Builder

	for _, l := range lines {
		if !strings.HasSuffix(l, ` = ""`) && !strings.HasSuffix(l, ` = false`) {

			sb.WriteString(l)
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil

}

func WrapFuncForPromptUI(f func(string) bool) func(string) error {
	return func(s string) error {
		if f(s) {
			return nil
		}
		return errors.New("not valid")
	}
}
