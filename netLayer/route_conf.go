package netLayer

import (
	"net"
	"net/netip"
	"reflect"
	"regexp"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/yl2chen/cidranger"
	"go.uber.org/zap"
)

type RuleConf struct {
	DialTag any `toml:"toTag" json:"toTag"`

	InTags []string `toml:"fromTag" json:"fromTag"`
	Users  []string `toml:"user" json:"user"`

	Countries []string `toml:"country" json:"country"` // 如果类似 !CN, 则意味着专门匹配不为CN 的国家（目前还未实现）
	IPs       []string `toml:"ip" json:"ip"`
	Domains   []string `toml:"domain" json:"domain"`
	Network   []string `toml:"network" json:"network"`
}

func (policy *RoutePolicy) LoadRulesForRoutePolicy(rules []*RuleConf) {
	for _, rc := range rules {
		policy.List = append(policy.List, LoadRuleForRouteSet(rc))
	}
}

func LoadRuleForRouteSet(rule *RuleConf) (rs *RouteSet) {
	if len(GeositeListMap) == 0 {
		err := LoadGeositeFiles()
		if err != nil {
			if ce := utils.CanLogErr("LoadGeositeFiles failed"); ce != nil {
				ce.Write(zap.Error(err), zap.String("Note", "You can use interactive-mode to download geosite files."))

			}
		}
	}
	rs = NewFullRouteSet()

	switch value := rule.DialTag.(type) {
	case string:
		rs.OutTag = value
	case []string:
		rs.OutTags = value
	case []any:
		list := make([]string, 0, len(value))
		for i, v := range value {
			if s, ok := v.(string); ok {
				list = append(list, s)
			} else {
				if ce := utils.CanLogErr("Route outTags list has not string type"); ce != nil {
					ce.Write(zap.Int("index", i), zap.String("type", reflect.TypeOf(v).String()), zap.Any("value", v))
				}
			}
		}
		rs.OutTags = list
	}

	for _, c := range rule.Countries {
		rs.Countries[c] = true
	}

	for _, d := range rule.Domains {
		colonIdx := strings.Index(d, ":")
		if colonIdx < 0 {
			rs.Match = append(rs.Match, d)

		} else {
			switch d[:colonIdx] {
			case "geosite":
				if GeositeListMap != nil {
					rs.Geosites = append(rs.Geosites, d[colonIdx+1:])

				}
			case "full":
				rs.Full[d[colonIdx+1:]] = true
			case "domain":
				rs.Domains[d[colonIdx+1:]] = true
			case "regexp":
				reg, err := regexp.Compile(d[colonIdx+1:])
				if err == nil {
					rs.Regex = append(rs.Regex, reg)
				} else {
					if ce := utils.CanLogErr("LoadRuleForRouteSet, regex illegal"); ce != nil {
						ce.Write(zap.Error(err))
					}
				}
			default:
				if ce := utils.CanLogErr("LoadRuleForRouteSet, not supported"); ce != nil {
					ce.Write(zap.String("item", d))
				}
			}

		}

		continue

	}

	for _, t := range rule.InTags {
		rs.InTags[t] = true
	}

	for _, u := range rule.Users {
		rs.Users[u] = true
	}

	//ip 过滤 需要 分辨 "private", cidr 和普通ip

	for _, ipStr := range rule.IPs {
		if ipStr == "private" {

			//https://www.arin.net/reference/research/statistics/address_filters/

			if _, net, err := net.ParseCIDR("10.0.0.0/8"); err == nil {
				rs.NetRanger.Insert(cidranger.NewBasicRangerEntry(*net))
			}
			if _, net, err := net.ParseCIDR("172.16.0.0/12"); err == nil {
				rs.NetRanger.Insert(cidranger.NewBasicRangerEntry(*net))
			}
			if _, net, err := net.ParseCIDR("192.168.0.0/16"); err == nil {
				rs.NetRanger.Insert(cidranger.NewBasicRangerEntry(*net))
			}

			continue
		}
		if strings.Contains(ipStr, "/") {
			if _, net, err := net.ParseCIDR(ipStr); err == nil {
				rs.NetRanger.Insert(cidranger.NewBasicRangerEntry(*net))
			}
			continue
		}

		na, e := netip.ParseAddr(ipStr)
		if e == nil {
			rs.IPs[na] = true
		} else {
			if ce := utils.CanLogErr("LoadRuleForRouteSet, parse ip failed"); ce != nil {
				ce.Write(zap.String("ipStr", ipStr), zap.Error(e))
			}
		}
	}

	if len(rule.Network) > 0 {
		rs.AllowedTransportLayerProtocols = 0 //因为 NewFullRouteSet 默认会同时允许 tcp和udp，所以在自定义网络层规则时，我们不用默认值。

		for _, netStr := range rule.Network {
			tp := StrToTransportProtocol(netStr)
			rs.AllowedTransportLayerProtocols |= tp
		}
	}

	return rs
}
