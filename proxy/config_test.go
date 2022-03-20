package proxy_test

import (
	"net/url"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestClientSimpleConfig(t *testing.T) {
	confstr1 := `{
	"local": "socks5://0.0.0.0:10800#taglocal",
	"remote": "vlesss://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4433?insecure=true&version=0#tag1",
	"mycountry":"CN",
	"fallbacks":[
    {
      "path":"/asf",
      "dest":6060
    }
  ]
}`

	mc, err := proxy.LoadSimpleConfigFromStr(confstr1)
	if err != nil {
		t.Log("loadConfigFromStr err", err)
		t.FailNow()
	}
	t.Log(mc.Client_ThatDialRemote_Url)
	u, e := url.Parse(mc.Client_ThatDialRemote_Url)
	if e != nil {
		t.FailNow()
	}
	t.Log(u.Fragment)

	u, e = url.Parse(mc.Server_ThatListenPort_Url)
	if e != nil {
		t.FailNow()
	}
	t.Log(u.Fragment)
	t.Log(mc.Server_ThatListenPort_Url)
	t.Log(mc.MyCountryISO_3166)
	if mc.MyCountryISO_3166 != "CN" {
		t.FailNow()
	}
	t.Log(mc.Fallbacks, len(mc.Fallbacks))
	for i, v := range mc.Fallbacks {
		t.Log(i, v)
	}
}

func TestTomlConfig(t *testing.T) {

	var conf proxy.Standard
	_, err := toml.Decode(testTomlConfStr, &conf)

	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Log(conf)
	t.Log(conf.Dial[0])
	t.Log(conf.Listen[0])
	t.Log(conf.Route[0])
	t.Log(conf.Route[1])
	t.Log(conf.Fallbacks)
}

const testTomlConfStr = `# this is a verysimple standard config

[app]
mycountry = "CN"

[[dial]]
tag = "my_vlesss1"
protocol = "vlesss"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = 4433
version = 0
insecure = true
utls = true

[[listen]]
protocol = "socks5"
host = "127.0.0.1"
port = 1080
tag = "my_socks51"

[[route]]
dialTag = "my_ws1"
country = ["CN"]
ip = ["0.0.0.0/8","10.0.0.0/8","fe80::/10","10.0.0.1"]
domain = ["www.google.com","www.twitter.com"]
network = ["tcp","udp"]

[[route]]
dialTag = "my_vless1"


[[fallback]]
path = "/asf"
dest = 6060
`
