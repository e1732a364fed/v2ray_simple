package httpLayer

import (
	"errors"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

const (
	Fallback_none = 0
)
const (
	FallBack_default byte = 1 << iota //default 其实也是path，只不过path是通配符。

	//这里剩余fallback按该固定顺序排列. 这是为了方便代码的书写(alpn和sni都是tls的)

	// 虽然alpn和sni都是tls的，但是回落本身是专门用于http的，所以还是放在httpLayer包中

	Fallback_path
	Fallback_alpn
	Fallback_sni

	fallback_end

	all_non_default_fallbacktype_count = iota - 2

	alpn_unspecified = 0
)

const (
	alpn_http11 = 1 << iota
	alpn_http20
)

var ErrShouldFallback = errors.New("will fallback")

func getfallbacktype_byindex(i int) byte {
	return 1 << (i + 1)
}

//判断 Fallback.SupportType 返回的 数值 是否具有特定的Fallback类型
func HasFallbackType(ftype, b byte) bool {
	return ftype&b > 0
}

//实现 Fallback. 这里的fallback只与http协议有关，所以只能按path,alpn 和 sni 进行分类
type Fallback interface {
	GetFallback(ftype byte, params ...string) *FallbackResult
	SupportType() byte //参考Fallback_开头的常量。如果支持多个，则返回它们 按位与 的结果
}

type FallbackResult struct {
	Addr netLayer.Addr
	Xver int
}

func (ef *FallbackResult) GetFallback(ftype byte, _ ...string) *FallbackResult {
	return ef
}

func (FallbackResult) SupportType() byte {
	return FallBack_default
}

type FallbackConf struct {
	//可选
	FromTag string `toml:"from" json:"from"` //which inServer triggered this fallback

	Xver int `toml:"xver" json:"xver"` //if fallback, whether to use PROXY protocol, and which version

	//必填
	Dest interface{} `toml:"dest" json:"dest"` //number port，or string "ip:port"

	//几种匹配方式，可选

	Path string   `toml:"path" json:"path"`
	Sni  string   `toml:"sni" json:"sni"`
	Alpn []string `toml:"alpn" json:"alpn"`
}

/*
// FallbackErr 可以在返回错误时，同时给定一个 指定的 Fallback
type FallbackErr interface {
	Error() string
	Fallback() Fallback
}

*/
