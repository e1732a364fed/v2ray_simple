package netLayer_test

import (
	"testing"

	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func TestDNS(t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog()

	config := `

	[dns]
servers = [
	"udp://114.114.114.114:53"
]

[dns.hosts]
"www.myfake.com" = "11.22.33.44"


`

	c, e := proxy.LoadTomlConfStr(config)
	if e != nil {
		t.Log(e)
		t.FailNow()
	}
	t.Log(c.DnsConf)

	dm := proxy.LoadDnsMachine(c.DnsConf)

	t.Log(&dm)
	t.Log(dm.DefaultConn.RemoteAddr().Network(), dm.DefaultConn.RemoteAddr())

	//dm.TypeStrategy = 60

	t.Log("record for  www.myfake.com is ", dm.Query("www.myfake.com"))

	t.Log("record for  www.qq.com is ", dm.Query("www.qq.com"))

	t.Log("record for  imgstat.baidu.com is ", dm.Query("imgstat.baidu.com"))
	t.Log("record for  imgstat.n.shifen.com is ", dm.Query("imgstat.n.shifen.com"))

}
