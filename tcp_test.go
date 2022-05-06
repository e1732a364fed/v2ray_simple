package v2ray_simple_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func TestTCP_vless(t *testing.T) {
	testTCP(t, "vless", 0, "tcp", false)
}

func TestTCP_trojan(t *testing.T) {
	testTCP(t, "trojan", 0, "tcp", false)
}

func TestTCP_trojan_mux(t *testing.T) {
	testTCP(t, "trojan", 0, "tcp", true)
}

//tcp测试我们直接使用http请求来测试
func testTCP(t *testing.T, protocol string, version int, network string, innermux bool) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog("")

	var testClientConfFormatStr = `
[[listen]]
protocol = "http"
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

	if innermux {
		testClientConfFormatStr += "use_mux = true"
	}

	const testServerConfFormatStr = `
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

	clientListenPort := netLayer.RandPortStr(true, false)
	clientDialPort := netLayer.RandPortStr(true, false)

	testClientConfStr := fmt.Sprintf(testClientConfFormatStr, clientListenPort, protocol, clientDialPort, version, network)

	testServerConfStr := fmt.Sprintf(testServerConfFormatStr, protocol, clientDialPort, version, network)

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

	//先建立服务端监听和客户端监听，最后自定义dns查询 并导向 客户端的 dokodemo监听端口

	//http in
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

	c1 := v2ray_simple.ListenSer(clientEndInServer, clientEndOutClient, nil)
	c2 := v2ray_simple.ListenSer(serverEndInServer, serverEndOutClient, nil)
	if c1 != nil {
		defer c1.Close()
	}
	if c2 != nil {
		defer c2.Close()
	}

	proxyurl := "http://127.0.0.1:" + clientListenPort

	url_proxy, e2 := url.Parse(proxyurl)
	if e2 != nil {
		fmt.Println("proxyurl given was wrong,", proxyurl, e2)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(url_proxy),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	tryGetHttp(t, client, "http://captive.apple.com")
	tryGetHttp(t, client, "http://www.msftconnecttest.com/connecttest.txt")

	//联通性测试 可参考 https://imldy.cn/posts/99d42f85/
	// 用这种 captive 测试 不容易遇到 网站无法在 某些地区 如 github action 所在的地区 访问 or卡顿等情况.
}

func tryGetHttp(t *testing.T, client *http.Client, path string) {
	t.Log("start dial", path)
	resp, err := client.Get(path)
	if err != nil {
		t.Log("get http failed", err)
		t.FailNow()
	}

	t.Log("Got response, start read")

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Log("get http failed read", err)
		t.FailNow()
	}
	resp.Body.Close()

	t.Log("got len", len(bs))
	if len(bs) > 5 {
		t.Log("first 5:", string(bs[:5]))

	} else {
		t.Log("all:", bs)

	}

}
