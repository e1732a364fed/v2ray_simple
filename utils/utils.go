// Package utils provides utilities that is used in all codes in verysimple
package utils

import (
	"errors"
	"flag"
	"runtime"
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

func IsFilePath(s string) error {

	//https://stackoverflow.com/questions/1976007/what-characters-are-forbidden-in-windows-and-linux-directory-names

	if runtime.GOOS == "windows" {
		if strings.ContainsAny(s, ":<>\"/\\|?*") {
			return errors.New("contain illegal characters")
		}
		if strings.ContainsAny(s, string([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31})) {
			return errors.New("contain illegal ASCII control characters")
		}
	} else {
		if strings.Contains(s, string([]byte{0})) {
			return errors.New("contain illegal characters")
		}
	}
	return nil
}
