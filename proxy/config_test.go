package proxy_test

import (
	"net/url"
	"testing"

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

	mc, hasE, err := proxy.LoadSimpleConfigFromStr(confstr1)
	if hasE {
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
