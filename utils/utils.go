// Package utils provides utilities that is used in all codes in verysimple
package utils

import (
	"errors"
	"flag"
	"strings"

	"github.com/BurntSushi/toml"
)

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
