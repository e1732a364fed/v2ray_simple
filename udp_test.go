package v2ray_simple_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/proxy/socks5"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/miekg/dns"
)

func TestUDP_vless(t *testing.T) {
	testUDP(t, "vless", 0, "tcp", false, false, false)
}

//v0 没有fullcone

func TestUDP_vless_v1(t *testing.T) {
	testUDP(t, "vless", 1, "tcp", false, false, false)
}

func TestUDP_vless_v1_fullcone(t *testing.T) {
	testUDP(t, "vless", 1, "tcp", false, true, false)
}

func TestUDP_vless_v1_udpMulti(t *testing.T) {
	testUDP(t, "vless", 1, "tcp", true, false, false)
}

func TestUDP_vless_v1_udpMulti_fullcone(t *testing.T) {
	testUDP(t, "vless", 1, "tcp", true, true, false)
}

func TestUDP_trojan(t *testing.T) {
	testUDP(t, "trojan", 0, "tcp", false, false, false)
}

func TestUDP_trojan_mux(t *testing.T) {
	testUDP(t, "trojan", 0, "tcp", false, false, true)
}

func TestUDP_trojan_fullcone(t *testing.T) {
	testUDP(t, "trojan", 0, "tcp", false, true, false)
}

func TestUDP_trojan_through_udp(t *testing.T) {
	testUDP(t, "trojan", 0, "udp", false, false, false)
}

// udp测试我们直接使用dns请求来测试.
func testUDP(t *testing.T, protocol string, version int, network string, multi bool, fullcone bool, mux bool) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog("")

	//同时监听两个dokodemo, 发向不同raddr, 这样就可以模拟 多raddr目标时的 情况
	//vless v1的udp_multi的dialfunc 需要单一 client 拨号多个raddr 才能被触发, 所以还要使用socks5测试两次

	isTestInChina := utils.IsTimezoneCN()

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
target = "udp://%s:53"

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
	if mux {
		testClientConfFormatStr += "\nuse_mux = true\n"
	}

	if multi {
		testClientConfFormatStr += "\nextra = { vless1_udp_multi = true }\n"
	}

	clientListenPort := netLayer.RandPortStr(true, true)
	clientListen2Port := netLayer.RandPortStr(true, true)
	clientDialPort := netLayer.RandPortStr(true, false)
	socks5Port, socks5PortStr := netLayer.RandPort_andStr(true, false)

	var secondDnsServerStr string
	if isTestInChina {
		secondDnsServerStr = "114.114.114.114"
	} else {
		secondDnsServerStr = "1.1.1.1"

	}

	testClientConfStr := fmt.Sprintf(testClientConfFormatStr, clientListenPort,
		clientListen2Port, secondDnsServerStr, socks5PortStr, protocol, clientDialPort, version, network)

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

	clientConf, err := proxy.LoadStandardConfFromTomlStr(testClientConfStr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	serverConf, err := proxy.LoadStandardConfFromTomlStr(testServerConfStr)
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

	c1 := v2ray_simple.ListenSer(clientEndInServer, clientEndOutClient, nil, nil)
	c2 := v2ray_simple.ListenSer(clientEndInServer2, clientEndOutClient, nil, nil)
	c3 := v2ray_simple.ListenSer(clientEndInServer3, clientEndOutClient, nil, nil)
	c4 := v2ray_simple.ListenSer(serverEndInServer, serverEndOutClient, nil, nil)

	if c1 != nil {
		defer c1.Close()
	}
	if c2 != nil {
		defer c2.Close()
	}

	if c3 != nil {
		defer c3.Close()
	}
	if c4 != nil {
		defer c4.Close()
	}

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

	if isTestInChina {
		socks5ClientConn.WriteUDP_Target.IP = net.IPv4(114, 114, 114, 114)
	} else {
		socks5ClientConn.WriteUDP_Target.IP = net.IPv4(1, 1, 1, 1)

	}

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
