package configAdapter

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

/*
To Quantumult X [server_local] string

quantumult X 只支持 vmess,trojan,shadowsocks,http 这四种协议.
See https://github.com/crossutility/Quantumult-X/blob/master/sample.conf

圈叉的配置，每一个协议的格式都略有不同，我们只能照着示例分情况处理。

同时，我们不支持里面的 fast-open 等选项。
*/
func ToQX(dc *proxy.DialConf) string {
	var sb strings.Builder

	sb.WriteString(dc.Protocol)
	sb.WriteByte('=')
	sb.WriteString(dc.GetAddrStrForListenOrDial())

	var trojan_or_http_tlsFunc = func() {
		if dc.TLS {
			sb.WriteString(", over-tls=true")

			if dc.Host != "" {
				sb.WriteString(", tls-host=")
				sb.WriteString(dc.Host)
			}
			sb.WriteString(", tls-verification=")

			if dc.Insecure {
				sb.WriteString("false")

			} else {
				sb.WriteString("true")
			}
		}
	}

	switch dc.Protocol {
	case "shadowsocks":
		//vs 中，ss的 加密方法可以在两个地方指定，一个是uuid中的method部分，一个是 EncryptAlgo
		//其中，uuid的method部分是必须要给出的

		ok, m, p := utils.CommonSplit(dc.Uuid, "method", "pass")
		if !ok {
			return "parsing error when split uuid to get method and pass"
		}
		ea := m
		if dc.EncryptAlgo != "" {
			ea = dc.EncryptAlgo
		}
		sb.WriteString(", method=")

		sb.WriteString(strings.ToLower(ea))
		sb.WriteString(", password=")
		sb.WriteString(p)

		var hasObfs bool = true
		var onlyTls bool

		if dc.AdvancedLayer == "ws" {
			if dc.TLS {
				sb.WriteString(", obfs=wss")
			} else {
				sb.WriteString(", obfs=ws")
			}

		} else if dc.TLS {
			sb.WriteString(", obfs=tls")
			onlyTls = true

		} else if dc.HttpHeader != nil {
			sb.WriteString(", obfs=http")

		} else {
			hasObfs = false

		}

		if hasObfs {
			if dc.Host != "" {
				sb.WriteString(", obfs-host=")
				sb.WriteString(dc.Host)

			}
			if !onlyTls {
				if dc.Path != "" {
					sb.WriteString(", obfs-uri=")
					sb.WriteString(dc.Path)
				} else if dc.HttpHeader != nil {
					if len(dc.HttpHeader.Request.Path) > 0 {
						sb.WriteString(", obfs-uri=")
						sb.WriteString(dc.HttpHeader.Request.Path[0])

					}
				}
			}
		}

	case "vmess":
		sb.WriteString(", method=")
		ea := dc.EncryptAlgo
		if ea == "" {
			ea = "none"
		}
		sb.WriteString(strings.ToLower(ea))
		sb.WriteString(", password=")
		sb.WriteString(dc.Uuid)

		var hasObfs bool

		if dc.AdvancedLayer == "ws" {
			if dc.TLS {
				sb.WriteString(", obfs=wss")
			} else {
				sb.WriteString(", obfs=ws")
			}
			hasObfs = true

		} else if dc.TLS {
			sb.WriteString(", obfs=over-tls")

			hasObfs = true

		}

		if hasObfs {
			if dc.Host != "" {
				sb.WriteString(", obfs-host=")
				sb.WriteString(dc.Host)

			}

			if dc.AdvancedLayer == "ws" {
				if dc.Path != "" {
					sb.WriteString(", obfs-uri=")
					sb.WriteString(dc.Path)
				}
			}
		}

	case "http":
		if dc.Uuid != "" {
			ok, u, p := utils.CommonSplit(dc.Uuid, "user", "pass")
			if !ok {
				return "parsing error when split uuid to get user and pass"
			}
			sb.WriteString(", username=")

			sb.WriteString(u)
			sb.WriteString(", password=")
			sb.WriteString(p)
		}
		trojan_or_http_tlsFunc()
	case "trojan":
		sb.WriteString(", password=")
		sb.WriteString(dc.Uuid)
		trojan_or_http_tlsFunc()
	} //switch

	if dc.Tag != "" {
		sb.WriteString(", tag=")
		sb.WriteString(dc.Tag)

	}

	return sb.String()
}

func FromQX(str string) (dc proxy.DialConf) {
	//qx 的配置应该是基本的逗号分隔值形式
	str = utils.StandardizeSpaces(str)
	strs := strings.Split(str, ",")

	for i, p := range strs {
		ss := strings.Split(p, "=")
		n := ss[0]
		v := ss[1]
		n = strings.TrimSpace(n)
		v = strings.TrimSpace(v)

		if i == 0 {
			dc.Protocol = n
			hostport := v
			host, port, err := net.SplitHostPort(hostport)
			if err != nil {
				fmt.Printf("FromQX: net.SplitHostPort err, %s\n", err.Error())
			} else {
				ip := net.ParseIP(host)
				if ip != nil {
					dc.IP = host
				} else {
					dc.Host = host
				}

				np, _ := strconv.Atoi(port)
				dc.Port = np
			}
		} else {
			switch n {
			case "method":
				dc.EncryptAlgo = v
			case "password":
				dc.Uuid = v
			case "tag":
				dc.Tag = v
			case "obfs-uri":
				dc.Path = v
			case "obfs-host":
				dc.Host = v
			case "udp-relay":
				if v == "false" {
					dc.Network = "tcp"
				}
			case "obfs":
				switch v {
				case "ws":
					dc.AdvancedLayer = "ws"
				case "wss":
					dc.AdvancedLayer = "ws"
					dc.TLS = true
				case "tls", "over-tls":
					dc.TLS = true
				}
			}
		}
	}

	if dc.Protocol == "shadowsocks" {
		if dc.Uuid != "" && dc.EncryptAlgo != "" {
			dc.Uuid = "method:" + dc.EncryptAlgo + "\n" + "pass:" + dc.Uuid
		}
	}
	return
}

/*
	clash使用yaml作为配置格式。本函数中不导出整个yaml配置文件，而只导出

对应的proxies项下的子项，比如

  - name: "ss1"
    type: ss
    server: server
    port: 443
    cipher: chacha20-ietf-poly1305
    password: "password"

See https://github.com/Dreamacro/clash/wiki/Configuration

clash的配置对于不同的协议来说，格式也有不同，clash基本上尊重了每一个协议的约定, 但也不完全一致
*/
func ToClash(dc *proxy.DialConf) string {
	//这里我们不使用外部yaml包，可以减少依赖.
	// 开发提示： 边照着实际yaml边 编写； 随时运行 go test以查看效果
	var sb strings.Builder
	sb.WriteString("  - name: ")
	if dc.Tag == "" {
		sb.WriteString(dc.Protocol)
	} else {
		sb.WriteString(dc.Tag)
	}
	sb.WriteString("\n    type: ")
	if dc.Protocol == "shadowsocks" {
		sb.WriteString("ss")
	} else {
		sb.WriteString(dc.Protocol)
	}
	sb.WriteString("\n    server: ")
	if dc.IP != "" {
		sb.WriteString(dc.IP)
	} else {
		sb.WriteString(dc.Host)
	}
	sb.WriteString("\n    port: ")
	sb.WriteString(strconv.Itoa(dc.Port))

	var writeHeaders = func() {
		if dc.Host != "" {
			sb.WriteString("\n      host: ")
			sb.WriteString(dc.Host)
		}
		if dc.Path != "" {
			sb.WriteString("\n      path: ")
			sb.WriteString(dc.Path)
		}
		if dc.HttpHeader != nil {
			if dc.HttpHeader.Request != nil {
				if len(dc.HttpHeader.Request.Headers) > 0 {
					sb.WriteString("\n      headers: ")

					for h, v := range dc.HttpHeader.Request.Headers {
						if len(v) > 0 {
							sb.WriteString("\n        ")
							sb.WriteString(h)
							sb.WriteString(": ")
							sb.WriteString(v[0])
						}
					}
				}
			}
		}
	}
	var ss_http_or_tls = func(mode string) {
		sb.WriteString("\n    plugin: obfs")
		sb.WriteString("\n    plugin-opts:")
		sb.WriteString("\n      mode: ")
		sb.WriteString(mode)
		if dc.Host != "" {
			sb.WriteString("\n      host: ")
			sb.WriteString(dc.Host)
		}
	}

	switch dc.Protocol {
	case "shadowsocks":
		ok, m, p := utils.CommonSplit(dc.Uuid, "method", "pass")
		if !ok {
			return "parsing error when split uuid to get method and pass"
		}
		sb.WriteString("\n    cipher: ")
		sb.WriteString(m)

		sb.WriteString("\n    password: ")
		sb.WriteString(p)

		if dc.AdvancedLayer == "ws" {
			sb.WriteString("\n    plugin: v2ray-plugin")
			sb.WriteString("\n    plugin-opts:")
			sb.WriteString("\n      mode: websocket")

			if dc.TLS {
				sb.WriteString("\n      tls: true")
				if dc.Insecure {
					sb.WriteString("\n      skip-cert-verify: true")
				}
			}

			writeHeaders()

		} else if dc.TLS {
			ss_http_or_tls("tls")
		} else if dc.HttpHeader != nil {
			ss_http_or_tls("http")
		}
	case "trojan":
		fallthrough
	case "vmess":
		if dc.Protocol == "vmess" {
			sb.WriteString("\n    uuid: ")
			sb.WriteString(dc.Uuid)
			sb.WriteString("\n    alterId: 0")
			sb.WriteString("\n    cipher: ")
			if dc.EncryptAlgo != "" {
				sb.WriteString(dc.EncryptAlgo)
			} else {
				sb.WriteString("auto")
			}

		} else {
			sb.WriteString("\n    password: ")
			sb.WriteString(dc.Uuid)
		}

		if dc.TLS {
			if dc.Protocol == "vmess" {
				sb.WriteString("\n    tls: true")
			}
			if dc.Insecure {
				sb.WriteString("\n    skip-cert-verify: true")
			}
		}
		if dc.Host != "" {
			if dc.Protocol == "vmess" {
				sb.WriteString("\n    servername: ")
			} else {
				sb.WriteString("\n    sni: ")
			}
			sb.WriteString(dc.Host)
		}
		if dc.HttpHeader != nil {
			sb.WriteString("\n    network: http")
			if r := dc.HttpHeader.Request; r != nil {
				sb.WriteString("\n    http-opts:")
				if r.Method != "" {
					sb.WriteString("\n      method: ")
					sb.WriteString(r.Method)

				}
				writeHeaders()
			}

		} else if dc.AdvancedLayer != "" {
			sb.WriteString("\n    network: ")
			sb.WriteString(dc.AdvancedLayer)

			switch dc.AdvancedLayer {
			case "ws":
				sb.WriteString("\n    ws-opts:")
				writeHeaders()
				if dc.IsEarly {
					sb.WriteString("\n      max-early-data: 2048")
					sb.WriteString("\n      early-data-header-name: Sec-WebSocket-Protocol")
				}
			case "grpc":
				sb.WriteString("\n    grpc-opts:")
				sb.WriteString("\n      grpc-service-name: ")
				sb.WriteString(dc.Path)

			}

		}
	case "http":
		fallthrough
	case "socks5":
		ok, u, p := utils.CommonSplit(dc.Uuid, "user", "pass")
		if !ok {
			return "parsing error when split uuid to get user and pass"
		}
		sb.WriteString("\n    username: ")
		sb.WriteString(u)
		sb.WriteString("\n    password: ")
		sb.WriteString(p)

		if dc.TLS {
			sb.WriteString("\n    tls: true")
			if dc.Insecure {
				sb.WriteString("\n    skip-cert-verify: true")
			}
		}
	} //switch

	return sb.String()
}

type V2rayNConfig struct {
	V        string `json:"v"`   //配置文件版本号,主要用来识别当前配置
	PS       string `json:"ps"`  //备注或别名
	Add      string `json:"add"` //地址IP或域名
	Port     string `json:"port"`
	ID       string `json:"id"`
	Security string `json:"scy"`
	Net      string `json:"net"`  //(tcp\kcp\ws\h2\quic)
	Type     string `json:"type"` //(none\http\srtp\utp\wechat-video) *tcp or kcp or QUIC
	Host     string `json:"host"`
	Path     string `json:"path"`
	Tls      string `json:"tls"`
	Sni      string `json:"sni"`
}

// See https://github.com/2dust/v2rayN/wiki/%E5%88%86%E4%BA%AB%E9%93%BE%E6%8E%A5%E6%A0%BC%E5%BC%8F%E8%AF%B4%E6%98%8E(ver-2)
func ToV2rayN(dc *proxy.DialConf) string {
	if dc.Protocol != "vmess" {
		return "ToV2rayN doesn't support any protocol other than vmess, you give " + dc.Protocol
	}

	vc := V2rayNConfig{
		V:        "2",
		PS:       dc.Tag,
		Add:      dc.IP,
		Port:     strconv.Itoa(dc.Port),
		ID:       dc.Uuid,
		Security: dc.EncryptAlgo,
		Host:     dc.Host,
		Sni:      dc.Host,
		Path:     dc.Path,
	}
	if vc.Add == "" {
		vc.Add = dc.Host
	}
	if dc.TLS {
		vc.Tls = "tls"
	}
	if dc.AdvancedLayer != "" {
		vc.Net = dc.AdvancedLayer
	} else {
		vc.Net = "tcp"
	}
	if dc.AdvancedLayer == "" && dc.HttpHeader != nil {
		vc.Type = "http"
	}
	bs, err := json.Marshal(vc)
	if err != nil {
		return err.Error()
	}

	return "vmess://" + base64.URLEncoding.EncodeToString(bs)

}

func ExtractQxRemoteServers(configContentStr string) {
	lines := strings.Split(configContentStr, "\n")
	for i, l := range lines {

		if strings.Contains(l, "[server_remote]") {
			fmt.Printf("got  [server_remote] field\n")

			l = lines[i+1]
			strs := strings.SplitN(l, ",", 2)
			l = strs[0]

			fmt.Printf("downloading %s\n", l)

			resp, err := http.DefaultClient.Get(l)

			if err != nil {
				fmt.Printf("Download failed %s\n", err.Error())
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("Download got bad status: %s\n", resp.Status)
				return
			}
			fmt.Printf("download ok, reading...\n")

			buf := utils.GetBuf()

			counter := &utils.DownloadPrintCounter{}

			fmt.Printf("\nread ok\n")

			io.Copy(buf, io.TeeReader(resp.Body, counter))
			ls := bytes.Split(buf.Bytes(), []byte("\n"))
			for _, v := range ls {
				fmt.Println(string(v))
			}
			return
		}
	}

}
