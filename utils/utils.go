// Package utils provides general utilities.
package utils

import (
	"flag"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// 本来可以直接用 fmt.Print, 但是那个Print多了一次到any的装箱，所以如果只
// 打印一个字符串的话，不妨直接调用 os.Stdout.WriteString(str)。
func PrintStr(str string) {
	os.Stdout.WriteString(str)
}

func IsFlagGiven(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

//flag包有个奇葩的缺点, 没法一下子获取所有的已经配置的参数, 只能遍历；
// 如果我们有大量的参数需要判断是否给出过, 那么不如先提取到到map里。
//
// 实际上flag包的底层也是用的一个map, 但是它是私有的, 而且我们也不宜用unsafe暴露出来.
func GetGivenFlags() (m map[string]*flag.Flag) {
	m = make(map[string]*flag.Flag)
	flag.Visit(func(f *flag.Flag) {
		m[f.Name] = f
	})

	return
}

var GivenFlags map[string]*flag.Flag

//call flag.Parse() and assign given flags to GivenFlags.
func ParseFlags() {
	flag.Parse()
	GivenFlags = GetGivenFlags()
}

//return kv pairs for GivenFlags
func GivenFlagKVs() (r map[string]string) {
	r = map[string]string{}

	for k, f := range GivenFlags {
		r[k] = f.Value.String()
	}
	return
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
		return ErrInvalidData
	}
}
