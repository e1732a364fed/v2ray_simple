/*
package configAdapter provides methods to convert proxy.ListenConf and proxy.DialConf to some 3rd party formats. It also defines some extra config formats used in vs.

对于第三方工具的配置, 支持 quantumultX, clash, 以及 v2rayN 的配置格式

参考 https://github.com/e1732a364fed/v2ray_simple/discussions/163

本包内的函数不支持 vs约定的 末尾+s即表示使用tls的用法, 所以调用函数之前要预处理一下。

本包依然秉持KISS原则，用最笨的代码、最少的依赖，实现最小的可执行文件大小以及最快的执行速度。
*/
package configAdapter

import (
	"net/url"
	"strconv"

	"github.com/e1732a364fed/v2ray_simple/proxy"
)

//convert proxy.DialConf to verysimple's official URL format.
// See https://github.com/e1732a364fed/v2ray_simple/discussions/163
func ToVS(cc *proxy.CommonConf, dc *proxy.DialConf) string {
	var u url.URL

	u.Scheme = cc.Protocol
	if cc.TLS {
		u.Scheme += "s"
	}

	u.User = url.User(cc.Uuid)
	if cc.IP != "" {
		u.Host = cc.IP + ":" + strconv.Itoa(cc.Port)
	} else {
		u.Host = cc.Host + ":" + strconv.Itoa(cc.Port)

	}
	q := u.Query()
	if cc.Network != "" {
		q.Add("network", cc.Network)

	}

	if cc.Fullcone {
		q.Add("fullcone", "true")
	}

	if cc.TLS {
		if cc.Insecure {
			q.Add("insecure", "true")
		}
		if dc != nil && dc.Utls {
			q.Add("utls", "true")
		}
	}
	if cc.Path != "" {
		q.Add("path", cc.Path)

	}

	if hh := cc.HttpHeader; hh != nil {

		q.Add("http", "true")

		if r := hh.Request; r != nil {

			if r.Method != "" {
				q.Add("http.method", r.Method)
			}
			if r.Version != "" {
				q.Add("http.version", r.Version)
			}

			for k, v := range r.Headers {
				vstr := ""
				for i, v2 := range v {
					vstr += v2
					if i != len(v)-1 {
						vstr += ", "
					}
				}
				q.Add("header."+k, vstr)
			}
		}
	}

	if cc.AdvancedLayer != "" {
		q.Add("adv", cc.AdvancedLayer)
	}

	if cc.EncryptAlgo != "" {
		q.Add("security", cc.EncryptAlgo)
	}

	u.RawQuery = q.Encode()
	if cc.Tag != "" {
		u.Fragment = cc.Tag

	}

	return u.String()
}
