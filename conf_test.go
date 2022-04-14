package main

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestLoadTomlConf(t *testing.T) {

	var conf StandardConf
	_, err := toml.Decode(testTomlConfStr, &conf)

	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Log(conf)
	t.Log("dial0", conf.Dial[0])
	t.Log("listen0", conf.Listen[0])
	t.Log("extra", conf.Listen[0].Extra)
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
extra = { ws_earlydata = 4096 }


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
