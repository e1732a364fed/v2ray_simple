package main

import (
	"fmt"
	"net"
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/proxy/socks5"
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

func TestUDP_vless(t *testing.T) {
	testUDP("vless", 0, "tcp", false, false, t)
}

//v0 没有fullcone

func TestUDP_vless_v1(t *testing.T) {
	testUDP("vless", 1, "tcp", false, false, t)
}

func TestUDP_vless_v1_fullcone(t *testing.T) {
	testUDP("vless", 1, "tcp", false, true, t)
}

func TestUDP_vless_v1_udpMulti(t *testing.T) {
	testUDP("vless", 1, "tcp", true, false, t)
}

func TestUDP_vless_v1_udpMulti_fullcone(t *testing.T) {
	testUDP("vless", 1, "tcp", true, true, t)
}

func TestUDP_trojan(t *testing.T) {
	testUDP("trojan", 0, "tcp", false, false, t)
}

func TestUDP_trojan_fullcone(t *testing.T) {
	testUDP("trojan", 0, "tcp", false, true, t)
}

func TestUDP_trojan_through_udp(t *testing.T) {
	testUDP("trojan", 0, "udp", false, false, t)
}

//经实测，udp dokodemo/socks5->vless/trojan (tcp/udp)->udp direct 来请求dns 是毫无问题的。
func testUDP(protocol string, version int, network string, multi bool, fullcone bool, t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog()

	//同时监听两个dokodemo, 发向不同raddr, 这样就可以模拟 多raddr目标时的 情况
	//vless v1的udp_multi的dialfunc 需要单一 client 拨号多个raddr 才能被触发, 所以还要使用socks5测试两次

	var testClientConfFormatStr = `
[[listen]]
protocol = "dokodemo"
network = "udp"
host = "127.0.0.1"
port = %s
target = "udp://8.8.8.8:53"

[[listen]]
protocol = "dokodemo"
network = "udp"
host = "127.0.0.1"
port = %s
target = "udp://114.114.114.114:53"

[[listen]]
protocol = "socks5"
host = "127.0.0.1"
port = %s

[[dial]]
protocol = "%s"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = %s
version = %d
insecure = true
network = "%s"
`
	if multi {
		testClientConfFormatStr += "\nextra = { vless1_udp_multi = true }"
	}

	clientListenPort := netLayer.RandPortStr()
	clientListen2Port := netLayer.RandPortStr()
	clientDialPort := netLayer.RandPortStr()
	socks5Port, socks5PortStr := netLayer.RandPort_andStr()

	testClientConfStr := fmt.Sprintf(testClientConfFormatStr, clientListenPort,
		clientListen2Port, socks5PortStr, protocol, clientDialPort, version, network)

	var testServerConfFormatStr = `
[[listen]]
protocol = "%s"
uuid = "a684455c-b14f-11ea-bf0d-42010aaa0003"
host = "127.0.0.1"
port = %s
version = %d
insecure = true
cert = "cert.pem"
key = "cert.key"
network = "%s"

[[dial]]
protocol = "direct"

`
	if fullcone {
		testServerConfFormatStr += "fullcone = true"

	}

	testServerConfStr := fmt.Sprintf(testServerConfFormatStr, protocol, clientDialPort, version, network)

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

	//domodemo in2
	clientEndInServer2, err := proxy.NewServer(clientConf.Listen[1])
	if err != nil {
		t.Log("can not create clientEndInServer: ", err)
		t.FailNow()
	}

	//socks5 in
	clientEndInServer3, err := proxy.NewServer(clientConf.Listen[2])
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
	listenSer(clientEndInServer2, clientEndOutClient, false)
	listenSer(clientEndInServer3, clientEndOutClient, false)
	listenSer(serverEndInServer, serverEndOutClient, false)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn("www.qq.com"), dns.TypeA)
	c := new(dns.Client)

	// server 1 测试 /////////////////////////////////////////
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

	// server 2 测试 /////////////////////////////////////////

	r, _, err = c.Exchange(m, "127.0.0.1:"+clientListen2Port)
	if r == nil {
		t.Log("test2, error: ", err.Error())
		t.FailNow()
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Log("test2, err2 ", r.Rcode, r)
		t.FailNow()
	}

	for _, a := range r.Answer {
		t.Log("test2, header is", a.Header())
		t.Log("test2, string is", a.String())
		t.Log("test2, a is ", a)

		if aa, ok := a.(*dns.A); ok {
			t.Log("test2, arecord is ", aa.A)
		}
	}

	// server 3 socks5 udp 测试 /////////////////////////////////////////
	//向不同地址写入两次，以测试 vless v1 udp multi

	socks5ClientConn := &socks5.ClientUDPConn{
		ServerAddr: &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: socks5Port,
		},
		WriteUDP_Target: &net.UDPAddr{
			IP:   net.IPv4(8, 8, 8, 8),
			Port: 53,
		},
	}
	err = socks5ClientConn.Associate()
	if err != nil {
		t.Log("test3, Associate error: ", err.Error())
		t.FailNow()
	}

	r, _, err = c.ExchangeWithConn(m, &dns.Conn{
		Conn: socks5ClientConn,
	})

	if r == nil {
		t.Log("test3, error: ", err.Error())
		t.FailNow()
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Log("test3, err2 ", r.Rcode, r)
		t.FailNow()
	}

	for _, a := range r.Answer {
		t.Log("test3, header is", a.Header())
		t.Log("test3, string is", a.String())
		t.Log("test3, a is ", a)

		if aa, ok := a.(*dns.A); ok {
			t.Log("test3, arecord is ", aa.A)
		}
	}

	socks5ClientConn.WriteUDP_Target.IP = net.IPv4(114, 114, 114, 114)

	r, _, err = c.ExchangeWithConn(m, &dns.Conn{
		Conn: socks5ClientConn,
	})

	if r == nil {
		t.Log("test3_2, error: ", err.Error())
		t.FailNow()
	}

	if r.Rcode != dns.RcodeSuccess {
		t.Log("test3_2, err2 ", r.Rcode, r)
		t.FailNow()
	}

	for _, a := range r.Answer {
		t.Log("test3_2, header is", a.Header())
		t.Log("test3_2, string is", a.String())
		t.Log("test3_2, a is ", a)

		if aa, ok := a.(*dns.A); ok {
			t.Log("test3_2, arecord is ", aa.A)
		}
	}
}
