/*
package configAdapter provides methods to convert proxy.ListenConf and proxy.DialConf to some 3rd party formats. It also defines some extra config formats used in vs.

计划支持 quantumultX, clash, 以及 v2rayN 的配置格式

参考 https://github.com/e1732a364fed/v2ray_simple/discussions/163
*/
package configAdapter

import (
	"strings"

	"github.com/e1732a364fed/v2ray_simple/proxy"
)

/*
quantumult X 只支持 vmess,trojan,shadowsocks,http 这四种协议.
See https://github.com/crossutility/Quantumult-X/blob/master/sample.conf
*/
func ToQX(dc *proxy.DialConf) string {
	var sb strings.Builder

	sb.WriteString(dc.Protocol)
	sb.WriteByte('=')
	sb.WriteString(dc.GetAddrStr())
	sb.WriteString(", ")

	switch dc.Protocol {
	case "vmess":
		sb.WriteString("method=")
		ea := dc.EncryptAlgo
		if ea == "" {
			ea = "none"
		}
		sb.WriteString(ea)
		sb.WriteString(", ")

	}
	sb.WriteString("password=")
	sb.WriteString(dc.Uuid)

	return sb.String()
}

func ToClash(dc *proxy.DialConf) string {

	return ""
}

func ToV2rayN(dc *proxy.DialConf) string {
	return ""
}
