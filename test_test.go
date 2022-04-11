package main

import (
	"fmt"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/miekg/dns"
)

/*
nc 模拟dns请求
https://unix.stackexchange.com/questions/600194/create-dns-query-with-netcat-or-dev-udp

echo cfc9 0100 0001 0000 0000 0000 0a64 7563 6b64 7563 6b67 6f03 636f 6d00 0001 0001 |
    xxd -p -r | nc -u -v 114.114.114.114 53

不过为了灵活我们还是引用 miekg/dns 包
	参考 https://zhengyinyong.com/post/go-dns-library/

虽然net.Resolver也能用，
https://stackoverflow.com/questions/59889882/specifying-dns-server-for-lookup-in-go

但是我还是喜欢 miekg/dns;

func TestDNSLookup_CN(t *testing.T) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.qq.com"), dns.TypeA)
	c := new(dns.Client)

	r, _, err := c.Exchange(m, "114.114.114.114:53")
	if r == nil {
		t.Log("*** error: ", err.Error())
		t.FailNow()
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Log("*** err2 ", r.Rcode, r)
		t.FailNow()
	}

	for _, a := range r.Answer {
		t.Log(a)
	}
}
*/

func TestUDP_dokodemo_vless(t *testing.T) {
	testUDP_dokodemo_protocol("vless", "tcp", t)
}

func TestUDP_dokodemo_trojan(t *testing.T) {
	testUDP_dokodemo_protocol("trojan", "tcp", t)
}

func TestUDP_dokodemo_trojan_through_udp(t *testing.T) {
	testUDP_dokodemo_protocol("trojan", "udp", t)
}

//经实测，udp dokodemo->vless/trojan (tcp/udp)->udp direct 来请求dns 是毫无问题的。
func testUDP_dokodemo_protocol(protocol string, network string, t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog()

	const testClientConfFormatStr = `
[[listen]]
protocol = "dokodemo"
network = "udp"
host = "127.0.0.1"
port = %s
target = "udp://8.8.8.8:53"

[[dial]]
protocol = "%s"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = %s
version = 0
insecure = true
network = "%s"
`
	clientListenPort := netLayer.RandPortStr()
	clientDialPort := netLayer.RandPortStr()

	testClientConfStr := fmt.Sprintf(testClientConfFormatStr, clientListenPort, protocol, clientDialPort, network)

	const testServerConfFormatStr = `
[[dial]]
protocol = "direct"

[[listen]]
protocol = "%s"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = %s
version = 0
insecure = true
cert = "cert.pem"
key = "cert.key"
network = "%s"
`

	testServerConfStr := fmt.Sprintf(testServerConfFormatStr, protocol, clientDialPort, network)

	clientConf, err := LoadTomlConfStr(testClientConfStr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	serverConf, err := LoadTomlConfStr(testServerConfStr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	//先建立服务端监听和客户端监听，最后自定义dns查询 并导向 客户端的 dokodemo监听端口

	//domodemo in
	clientEndInServer, err := proxy.NewServer(clientConf.Listen[0])
	if err != nil {
		t.Log("can not create clientEndInServer: ", err)
		t.FailNow()
	}

	// vless out
	clientEndOutClient, err := proxy.NewClient(clientConf.Dial[0])
	if err != nil {
		t.Log("can not create clientEndOutClient: ", err)
		t.FailNow()
	}

	//vless in
	serverEndInServer, err := proxy.NewServer(serverConf.Listen[0])
	if err != nil {
		t.Log("can not create serverEndInServer: ", err)
		t.FailNow()
	}
	// direct out
	serverEndOutClient, err := proxy.NewClient(serverConf.Dial[0])
	if err != nil {
		t.Log("can not create serverEndOutClient: ", err)
		t.FailNow()
	}

	listenSer(clientEndInServer, clientEndOutClient, false)
	listenSer(serverEndInServer, serverEndOutClient, false)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.qq.com"), dns.TypeA)
	c := new(dns.Client)

	r, _, err := c.Exchange(m, "127.0.0.1:"+clientListenPort)
	if r == nil {
		t.Log("error: ", err.Error())
		t.FailNow()
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Log("err2 ", r.Rcode, r)
		t.FailNow()
	}

	for _, a := range r.Answer {
		t.Log("header is", a.Header())
		t.Log("string is", a.String())
		t.Log("a is ", a)

		if aa, ok := a.(*dns.A); ok {
			t.Log("arecord is ", aa.A)
		}
	}
}

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
