package configAdapter

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// Generate shadowsocks's official uri based on proxy.CommonConf and other parameters.
//
// See https://github.com/shadowsocks/shadowsocks-org/wiki/SIP002-URI-Scheme
// 若lc给出，则表示用于服务端.
// sip 用于指定协议标准，可以设为4 或者22. 4 表示 sip004, Aead; 22表示 sip022, 即aead-2022
func ToSS(cc *proxy.CommonConf, lc *proxy.ListenConf, plain_userinfo bool, sip int) string {

	// 不同于其他官方定义的分享协议，ss官方定义的url是既可以用于服务端，也可以用于客户端的，这个理念与vs的极简/命令行模式一致。

	/*

		SS-URI = "ss://" userinfo "@" hostname ":" port [ "/" ] [ "?" plugin ] [ "#" tag ]
		userinfo = websafe-base64-encode-utf8(method  ":" password)
		           method ":" password

		Note that encoding userinfo with Base64URL is recommended but optional for Stream and AEAD (SIP004). But for AEAD-2022 (SIP022), userinfo MUST NOT be encoded with Base64URL. When userinfo is not encoded, method and password MUST be percent encoded.

		The last / should be appended if plugin is present, but is optional if only tag is present
	*/
	var u url.URL

	isServer := lc != nil

	u.Scheme = cc.Protocol

	ok, m, p := utils.CommonSplit(cc.Uuid, "method", "pass")
	if !ok {
		return "parsing error when split uuid to get method and pass"

	}

	if sip == 22 {
		m = url.QueryEscape(m)
		p = url.QueryEscape(p)
		u.User = url.UserPassword(m, p)

	} else {
		if plain_userinfo {
			u.User = url.UserPassword(m, p)

		} else {

			u.User = url.User(base64.URLEncoding.EncodeToString([]byte(m + ":" + p)))

		}
	}

	if cc.IP != "" {
		u.Host = cc.IP + ":" + strconv.Itoa(cc.Port)
	} else {
		u.Host = cc.Host + ":" + strconv.Itoa(cc.Port)

	}

	q := u.Query()

	//https://github.com/shadowsocks/v2ray-plugin/blob/master/main.go
	switch cc.AdvancedLayer {
	case "ws":
		pluginStr := "v2ray-plugin"
		if isServer {
			pluginStr += ";server"

		}

		if cc.TLS {
			pluginStr += ";tls"
			if cc.Host != "" {
				pluginStr += ";host=" + cc.Host
			}
		}
		if cc.Path != "" {
			pluginStr += ";path=" + cc.Path
		}

		q.Add("plugin", pluginStr)
	case "quic":
		pluginStr := "v2ray-plugin"
		if isServer {
			pluginStr += ";server"

		}
		pluginStr += ";mode=quic"
		if cc.Host != "" {
			pluginStr += ";host=" + cc.Host
		}
		q.Add("plugin", pluginStr)

	default:
		//https://github.com/shadowsocks/simple-obfs
		//https://github.com/shadowsocks/simple-obfs/blob/486bebd9208539058e57e23a12f23103016e09b4/src/local.c
		if cc.HttpHeader != nil || cc.TLS {
			var pluginStr string

			obfs := "http"
			if cc.TLS {
				obfs = "tls"
			}

			if lc != nil {

				pluginStr = "obfs-server;obfs=" + obfs

			} else {
				pluginStr = "obfs-local;obfs=" + obfs
			}
			if cc.Host != "" {
				pluginStr += ";obfs-host=" + cc.Host
			}
			if cc.Path != "" {
				pluginStr += ";obfs-uri=" + cc.Path

			}
			if isServer && lc.Fallback != nil {
				switch value := lc.Fallback.(type) {
				case string:
					pluginStr += ";failover=" + value

				}
			}
			q.Add("plugin", pluginStr)

		}
	}

	u.RawQuery = q.Encode()
	if len(u.RawQuery) > 0 {
		u.Path = "/"
	}

	if cc.Tag != "" {
		u.Fragment = cc.Tag

	}

	return u.String()
}

// Generate xray url draft based on proxy.DialConf.
// See https://github.com/XTLS/Xray-core/discussions/716
func ToXray(dc *proxy.DialConf) string {
	//内容基本与 proxy/vless 包内的 GenerateXrayShareURL 一致, 为了不依赖vless包, 我们在这里并不复用代码

	var u url.URL

	u.Scheme = dc.Protocol
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
