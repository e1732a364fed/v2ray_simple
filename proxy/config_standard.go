package proxy

import (
	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

//配置文件格式
const (
	SimpleMode = iota
	StandardMode
	V2rayCompatibleMode

	ErrStrNoListenUrl = "no listen URL provided"
)

//标准配置，使用toml格式。
// toml：https://toml.io/cn/
//
// English: https://toml.io/en/
type StandardConf struct {
	DnsConf *netLayer.DnsConf `toml:"dns"`

	Listen []*ListenConf `toml:"listen"`
	Dial   []*DialConf   `toml:"dial"`

	Route     []*netLayer.RuleConf      `toml:"route"`
	Fallbacks []*httpLayer.FallbackConf `toml:"fallback"`
}

//convenient function for loading StandardConf from a string
func LoadStandardConfFromTomlStr(str string) (c StandardConf, err error) {
	_, err = toml.Decode(str, &c)
	return
}
