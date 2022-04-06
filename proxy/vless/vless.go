// Package vless provies vless proxy support for proxy.Client and proxy.Server
package vless

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

const Name = "vless"

const (
	CRUMFURS_ESTABLISHED byte = 20

	CRUMFURS_Established_Str = "CRUMFURS_Established"
)

// CMD types, for vless and vmess
const (
	_ byte = iota
	CmdTCP
	CmdUDP
	CmdMux
	Cmd_CRUMFURS //byte = 4 // start from vless v1

)

//https://github.com/XTLS/Xray-core/discussions/716
func GenerateXrayShareURL(dialconf *proxy.DialConf) string {

	var u url.URL

	u.Scheme = "vless"
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

	return u.String()
}
