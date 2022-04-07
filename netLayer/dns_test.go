package netLayer_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

type testConfStruct struct {
	DnsConf *netLayer.DnsConf `toml:"dns"`
}

func testDns_withConf(t *testing.T, config string) {

	utils.LogLevel = utils.Log_debug
	utils.InitLog()

	config += `
	[dns.hosts]
"www.myfake.com" = "11.22.33.44"
	`
	var c testConfStruct
	_, e := toml.Decode(config, &c)

	if e != nil {
		t.Log(e)
		t.FailNow()
	}
	t.Log(c.DnsConf)

	dm := netLayer.LoadDnsMachine(c.DnsConf)

	t.Log(&dm)

	//dm.TypeStrategy = 60

	t.Log("record for  www.myfake.com is ", dm.Query("www.myfake.com"))

	t.Log("record for  www.qq.com is ", dm.Query("www.qq.com"))

	t.Log("record for  imgstat.baidu.com is ", dm.Query("imgstat.baidu.com"))
	t.Log("record for  imgstat.n.shifen.com is ", dm.Query("imgstat.n.shifen.com"))
}

func TestDNS(t *testing.T) {
	const config = `
	[dns]
servers = [
	"udp://114.114.114.114:53"
]
`
	testDns_withConf(t, config)
}

func TestDNS_DoT(t *testing.T) {
	const config = `
	[dns]
servers = [
	"tls://223.5.5.5:853"
]
`
	testDns_withConf(t, config)

}

func TestDNS_SpecialServer(t *testing.T) {
	const config = `
	[dns]
servers = [
	{ addr = "udp://8.8.8.8:53", domain = [ "google.com" ] }
]
`
	utils.LogLevel = utils.Log_debug
	utils.InitLog()

	var c testConfStruct
	_, e := toml.Decode(config, &c)

	if e != nil {
		t.Log(e)
		t.FailNow()
	}
	t.Log(c.DnsConf)

	dm := netLayer.LoadDnsMachine(c.DnsConf)

	t.Log(&dm)

	//dm.TypeStrategy = 60

	t.Log("record for  google.com is ", dm.Query("google.com"))

}
