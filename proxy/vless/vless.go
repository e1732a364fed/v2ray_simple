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

func GenerateXrayShareURL(dialconf *proxy.DialConf) string {

	var u url.URL

	u.Scheme = Name
	u.User = url.User(dialconf.Uuid)
	if dialconf.IP != "" {
		u.Host = dialconf.IP + ":" + strconv.Itoa(dialconf.Port)
	} else {
		u.Host = dialconf.Host + ":" + strconv.Itoa(dialconf.Port)

	}
	q := u.Query()
	if dialconf.TLS {
		q.Add("security", "tls")
		if dialconf.Host != "" {
			q.Add("sni", dialconf.Host)

		}
		if len(dialconf.Alpn) > 0 {
			var sb strings.Builder
			for i, s := range dialconf.Alpn {
				sb.WriteString(s)
				if i != len(dialconf.Alpn)-1 {
					sb.WriteString(",")
				}
			}
			q.Add("alpn", sb.String())

		}

		//if dialconf.Insecure{
		//	q.Add("allowInsecure", true)

		//}
	}
	if dialconf.AdvancedLayer != "" {
		q.Add("type", dialconf.AdvancedLayer)

		switch dialconf.AdvancedLayer {
		case "ws":
			if dialconf.Path != "" {
				q.Add("path", dialconf.Path)
			}
			if dialconf.Host != "" {
				q.Add("host", dialconf.Host)

			}
		case "grpc":
			if dialconf.Path != "" {
				q.Add("serviceName", dialconf.Path)
			}
		}
	}

	u.RawQuery = q.Encode()
	if dialconf.Tag != "" {
		u.Fragment = dialconf.Tag

	}

	return u.String()
}
