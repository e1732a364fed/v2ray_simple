package proxy

import (
	"time"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

//used in real relay progress. See source code of v2ray_simple for details.
type RoutingEnv struct {
	RoutePolicy   *netLayer.RoutePolicy
	MainFallback  *httpLayer.ClassicFallback
	DnsMachine    *netLayer.DNSMachine
	ClientsTagMap map[string]Client //用于分流到某个tag的Client, 所以需要知道所有的client
}

func LoadEnvFromStandardConf(standardConf *StandardConf) (routingEnv RoutingEnv, Default_uuid string) {

	routingEnv.ClientsTagMap = make(map[string]Client)

	if len(standardConf.Fallbacks) != 0 {
		routingEnv.MainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)
	}

	if dnsConf := standardConf.DnsConf; dnsConf != nil {
		routingEnv.DnsMachine = netLayer.LoadDnsMachine(dnsConf)
	}

	var hasAppLevelMyCountry bool

	if appConf := standardConf.App; appConf != nil {

		Default_uuid = appConf.DefaultUUID

		hasAppLevelMyCountry = appConf.MyCountryISO_3166 != ""

		if appConf.UDP_timeout != nil {
			minutes := *appConf.UDP_timeout
			if minutes > 0 {
				netLayer.UDP_timeout = time.Minute * time.Duration(minutes)
			}
		}
	}

	if standardConf.Route != nil || hasAppLevelMyCountry {

		netLayer.LoadMaxmindGeoipFile("")

		routingEnv.RoutePolicy = netLayer.NewRoutePolicy()
		if hasAppLevelMyCountry {
			routingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

		}

		netLayer.LoadRulesForRoutePolicy(standardConf.Route, routingEnv.RoutePolicy)
	}

	return
}
