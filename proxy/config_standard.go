package proxy

import (
	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 配置文件格式
const (
	SimpleMode = iota
	StandardMode
	V2rayCompatibleMode

	ErrStrNoListenUrl = "no listen URL provided"
)

// 标准配置，使用toml格式。
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

// 将一些较长的配置项 以较短的缩写 作为同义词. 然后代码读取时 只使用短的词.
var StandardConfSynonyms = [][2]string{
	{"advancedLayer", "adv"},
	{"tls_rejectUnknownSni", "rejectUnknownSni"},
	{"utls = true", `tls_type = "utls"`},
}

var StandardConfBytesSynonyms [][2][]byte

func init() {
	for _, ss := range StandardConfSynonyms {

		StandardConfBytesSynonyms = append(StandardConfBytesSynonyms, [2][]byte{[]byte(ss[0]), []byte(ss[1])})
	}
}

// convenient function for loading StandardConf from a string. Calls utils.ReplaceStringsSynonyms(str, StandardConfSynonyms)
func LoadStandardConfFromTomlStr(str string) (c StandardConf, err error) {

	str = utils.ReplaceStringsSynonyms(str, StandardConfSynonyms)

	_, err = toml.Decode(str, &c)
	return
}
