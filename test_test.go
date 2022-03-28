package main

import (
	"log"
	"testing"

	"github.com/hahahrfool/v2ray_simple/proxy"
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
		log.Fatalln("*** error: ", err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		log.Fatalln("*** err2 ", r.Rcode, r)
	}

	for _, a := range r.Answer {
		t.Log(a)
	}
}
*/

//经实测，dokodemo->vless->udp 来请求dns是毫无问题的。
func TestUDP(t *testing.T) {

	const testClientConfStr = `
[[listen]]
protocol = "dokodemo"
network = "udp"
host = "127.0.0.1"
port = 1080
target = "udp://114.114.114.114:53"


[[dial]]
protocol = "vless"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = 4433
version = 0
insecure = true
`

	const testServerConfStr = `
[[dial]]
protocol = "direct"

[[listen]]
protocol = "vless"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = 4433
version = 0
insecure = true
cert = "cert.pem"
key = "cert.key"
`

	clientConf, err := proxy.LoadTomlConfStr(testClientConfStr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	serverConf, err := proxy.LoadTomlConfStr(testServerConfStr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	//先建立服务端监听，再建立客户端监听，最后自定义dns查询 并导向 客户端的 dokodemo监听端口

	//vless
	serverEndInServer, err := proxy.NewServer(serverConf.Listen[0])
	if err != nil {
		log.Fatalln("can not create local server: ", err)
	}
	// direct
	serverEndOutClient, err := proxy.NewClient(serverConf.Dial[0])
	if err != nil {
		log.Fatalln("can not create local server: ", err)
	}

	//domodemo
	clientEndInServer, err := proxy.NewServer(clientConf.Listen[0])
	if err != nil {
		log.Fatalln("can not create local server: ", err)
	}
	// vless
	clientEndOutClient, err := proxy.NewClient(clientConf.Dial[0])
	if err != nil {
		log.Fatalln("can not create local server: ", err)
	}

	listenSer(clientEndInServer, clientEndOutClient)
	listenSer(serverEndInServer, serverEndOutClient)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.qq.com"), dns.TypeA)
	c := new(dns.Client)

	r, _, err := c.Exchange(m, "127.0.0.1:1080")
	if r == nil {
		log.Fatalln("*** error: ", err.Error())
	}

	if r.Rcode != dns.RcodeSuccess {
		log.Fatalln("*** err2 ", r.Rcode, r)
	}

	for _, a := range r.Answer {
		t.Log(a)
	}
}
