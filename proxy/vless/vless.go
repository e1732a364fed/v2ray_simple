/* Package vless implements vless v0/v1 for proxy.Client and proxy.Server.


vless的客户端配置 分享url文档：
https://github.com/XTLS/Xray-core/discussions/716

*/
package vless

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const Name = "vless"

const (
	addon_udp_multi_flag = 1 // for v1
)

// CMD types, for vless and vmess
const (
	_ byte = iota
	CmdTCP
	CmdUDP
	CmdMux
)

//依照 vless 协议的格式 依次写入 地址的 port, 域名/ip 信息
func WriteAddrTo(writeBuf utils.ByteWriter, raddr netLayer.Addr) {
	writeBuf.WriteByte(byte(raddr.Port >> 8))
	writeBuf.WriteByte(byte(raddr.Port << 8 >> 8))
	abs, atyp := raddr.AddressBytes()
	writeBuf.WriteByte(atyp)
	writeBuf.Write(abs)

}

func GenerateXrayShareURL(dc *proxy.DialConf) string {

	var u url.URL

	u.Scheme = Name
	u.User = url.User(dc.Uuid)
	if dc.IP != "" {
		u.Host = dc.IP + ":" + strconv.Itoa(dc.Port)
	} else {
		u.Host = dc.Host + ":" + strconv.Itoa(dc.Port)

	}
	q := u.Query()
	if dc.TLS {
		q.Add("security", "tls")
		if dc.Host != "" {
			q.Add("sni", dc.Host)

		}
		if len(dc.Alpn) > 0 {
			var sb strings.Builder
			for i, s := range dc.Alpn {
				sb.WriteString(s)
				if i != len(dc.Alpn)-1 {
					sb.WriteString(",")
				}
			}
			q.Add("alpn", sb.String())

		}

		//if dialconf.Insecure{
		//	q.Add("allowInsecure", true)

		//}
	}
	if dc.AdvancedLayer != "" {
		q.Add("type", dc.AdvancedLayer)

		switch dc.AdvancedLayer {
		case "ws":
			if dc.Path != "" {
				q.Add("path", dc.Path)
			}
			if dc.Host != "" {
				q.Add("host", dc.Host)

			}
		case "grpc":
			if dc.Path != "" {
				q.Add("serviceName", dc.Path)
			}
		}
	}

	u.RawQuery = q.Encode()
	if dc.Tag != "" {
		u.Fragment = dc.Tag

	}

	return u.String()
}
