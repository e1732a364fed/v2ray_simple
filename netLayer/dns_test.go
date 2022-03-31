package netLayer_test

import (
	"testing"

	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/miekg/dns"
)

func TestDNS(t *testing.T) {
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

	t.Log("record for  www.myfake.com is ", dm.Query("www.myfake.com", dns.TypeA))

	t.Log("record for  www.qq.com is ", dm.Query("www.qq.com", dns.TypeA))
}
