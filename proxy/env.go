package proxy

import (
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

//used in real relay progress. See source code of v2ray_simple for details.
type RoutingEnv struct {
	RoutePolicy  *netLayer.RoutePolicy
	MainFallback *httpLayer.ClassicFallback
	DnsMachine   *netLayer.DNSMachine

	ClientsTagMap      map[string]Client //用于分流到某个tag的Client, 所以需要知道所有的client
	ClientsTagMapMutex sync.RWMutex
}

func (re *RoutingEnv) GetClient(tag string) (c Client) {
	re.ClientsTagMapMutex.RLock()

	c = re.ClientsTagMap[tag]
	re.ClientsTagMapMutex.RUnlock()
	return
}
func (re *RoutingEnv) SetClient(tag string, c Client) {
	re.ClientsTagMapMutex.Lock()

	re.ClientsTagMap[tag] = c
	re.ClientsTagMapMutex.Unlock()
}
func (re *RoutingEnv) DelClient(tag string) {
	re.ClientsTagMapMutex.Lock()

	delete(re.ClientsTagMap, tag)
	re.ClientsTagMapMutex.Unlock()
}

func LoadEnvFromStandardConf(standardConf *StandardConf) (routingEnv RoutingEnv) {

	routingEnv.ClientsTagMap = make(map[string]Client)

	if len(standardConf.Fallbacks) != 0 {
		routingEnv.MainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)
	}

	if dnsConf := standardConf.DnsConf; dnsConf != nil {
		routingEnv.DnsMachine = netLayer.LoadDnsMachine(dnsConf)
	}

	var hasAppLevelMyCountry bool

	if appConf := standardConf.App; appConf != nil {

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

		rp := netLayer.NewRoutePolicy()
		if hasAppLevelMyCountry {
			rp.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

		}

		rp.LoadRulesForRoutePolicy(standardConf.Route)

		routingEnv.RoutePolicy = rp

	}

	return
}
