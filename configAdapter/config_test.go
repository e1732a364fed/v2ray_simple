package configAdapter_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/configAdapter"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var myvmess_wss = &proxy.DialConf{
	CommonConf: proxy.CommonConf{
		Protocol:      "vmess",
		EncryptAlgo:   "aes-128-gcm",
		UUID:          utils.ExampleUUID,
		TLS:           true,
		AdvancedLayer: "ws",
		Path:          "/path1",
		IP:            "1.1.1.1",
		Port:          443,
		Host:          "example.com",
		Tag:           "myvmess_wss",
	},
}

var myss_http = &proxy.DialConf{
	CommonConf: proxy.CommonConf{
		Protocol:    "shadowsocks",
		EncryptAlgo: "chacha20",
		UUID:        "method:chacha20\npass:" + utils.ExampleUUID,
		HttpHeader: &httpLayer.HeaderPreset{
			Request: &httpLayer.RequestHeader{
				Path: []string{"/pathx"},
				Headers: map[string][]string{
					"custom1": {"value1"},
				},
			},
		},
		IP:   "1.1.1.1",
		Port: 443,
		Host: "example.com",
		Tag:  "my",
	},
}

var myss_wss = &proxy.DialConf{
	CommonConf: proxy.CommonConf{
		Protocol:    "shadowsocks",
		EncryptAlgo: "chacha20",
		UUID:        "method:chacha20\npass:" + utils.ExampleUUID,
		HttpHeader: &httpLayer.HeaderPreset{
			Request: &httpLayer.RequestHeader{
				Path: []string{"/pathx"},
				Headers: map[string][]string{
					"custom1": {"value1"},
				},
			},
		},
		TLS:           true,
		AdvancedLayer: "ws",
		Path:          "/path1",
		IP:            "1.1.1.1",
		Port:          443,
		Host:          "example.com",
		Tag:           "my",
	},
}

// unexhaustive
func TestToQX(t *testing.T) {

	if configAdapter.ToQX(myvmess_wss) != "vmess=1.1.1.1:443, method=aes-128-gcm, password=a684455c-b14f-11ea-bf0d-42010aaa0003, obfs=wss, obfs-host=example.com, obfs-uri=/path1, tag=myvmess_wss" {
		t.FailNow()
	}

	if configAdapter.ToQX(myss_http) != "shadowsocks=1.1.1.1:443, method=chacha20, password=a684455c-b14f-11ea-bf0d-42010aaa0003, obfs=http, obfs-host=example.com, obfs-uri=/pathx, tag=my" {
		t.FailNow()
	}
}

func TestToClash(t *testing.T) {
	t.Log(configAdapter.ToClash(myss_wss))
}

func TestToV2rayN(t *testing.T) {
	t.Log(configAdapter.ToV2rayN(myvmess_wss))
}

func TestToSS(t *testing.T) {
	t.Log(configAdapter.ToSS(&myss_wss.CommonConf, nil, false, 22))
}
