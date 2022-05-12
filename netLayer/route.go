package netLayer

import (
	"math/rand"
	"net/netip"
	"regexp"
	"strings"

	"github.com/yl2chen/cidranger"
)

//用于 HasFullOrSubDomain函数
type DomainHaser interface {
	HasDomain(string) bool
}

type MapDomainHaser map[string]bool

func (mdh MapDomainHaser) HasDomain(d string) bool {
	_, found := mdh[d]
	return found
}

//会以点号分裂domain判断每一个子域名是否被包含，最终会试图匹配整个字符串.
func HasFullOrSubDomain(domain string, ds DomainHaser) bool {
	lastDotIndex := len(domain)

	var suffix string
	for {

		lastDotIndex = strings.LastIndex(domain[:lastDotIndex], ".")

		suffix = domain[lastDotIndex+1:]
		if ds.HasDomain(suffix) {
			return true
		}
		if lastDotIndex == -1 {
			return false
		}
	}

}

// TargetDescription 可以完整地描述一个网络层/传输层上的一个特定目标,
// 一般来说，一个具体的监听配置就会分配一个tag
type TargetDescription struct {
	Addr  Addr
	InTag string

	UserIdentityStr string
}

// Set 是 “集合” 的意思, 是一组相同类型的数据放到一起。
//  这里的相同点，就是它们同属于 将发往一个方向, 即同属一个路由策略。
// 任意一个参数匹配后，都将发往相同的方向，由该方向OutTag 指定。
// RouteSet 只负责把一些属性相同的 “网络层/传输层 特征” 放到一起。
//
//这里主要通过 ip，域名 和 inTag 进行分流。域名的匹配又分多种方式。
type RouteSet struct {
	//网络层
	NetRanger cidranger.Ranger    //一个范围
	IPs       map[netip.Addr]bool //一个确定值

	//Domains匹配子域名，当此域名是目标域名或其子域名时，该规则生效.
	Domains map[string]bool

	//Full只匹配完整域名;
	Full   map[string]bool
	InTags map[string]bool

	//Countries 使用 ISO 3166 字符串 作为key.
	Countries map[string]bool

	//Users 包含所有可匹配的 用户的 identityStr
	Users map[string]bool

	//Regex是正则匹配域名.
	Regex []*regexp.Regexp

	//Match 匹配任意字符串
	Match, Geosites []string

	//传输层
	AllowedTransportLayerProtocols uint16

	OutTag  string   //目标
	OutTags []string //目标列表

}

func NewRouteSetForMyCountry(iso string) *RouteSet {
	if len(iso) != 2 {
		return nil
	}
	rs := &RouteSet{
		Countries:                      make(map[string]bool),
		Domains:                        make(map[string]bool),
		OutTag:                         "direct",
		AllowedTransportLayerProtocols: TCP | UDP, //默认即支持tcp和udp

	}
	rs.Countries[strings.ToUpper(iso)] = true
	rs.Domains[strings.ToLower(iso)] = true //iso字符串的小写正好可以作为顶级域名
	return rs
}

func NewFullRouteSet() *RouteSet {
	return &RouteSet{
		NetRanger:                      cidranger.NewPCTrieRanger(),
		IPs:                            make(map[netip.Addr]bool),
		Match:                          make([]string, 0),
		Domains:                        make(map[string]bool),
		Full:                           make(map[string]bool),
		Users:                          make(map[string]bool),
		Geosites:                       make([]string, 0),
		InTags:                         make(map[string]bool),
		Countries:                      make(map[string]bool),
		AllowedTransportLayerProtocols: TCP | UDP, //默认即支持tcp和udp
	}
}

func (sg *RouteSet) IsIn(td *TargetDescription) bool {
	var tagOk bool
	if len(sg.InTags) > 0 {
		if td.InTag != "" {
			_, tagOk = sg.InTags[td.InTag]
		}
	} else {
		tagOk = true
	}
	if !tagOk {
		return false
	}

	var userOk bool

	if len(sg.Users) > 0 {
		if td.UserIdentityStr != "" {
			_, userOk = sg.Users[td.UserIdentityStr]
		}
	} else {
		userOk = true
	}

	if !userOk {
		return false
	}

	if sg.IsNoLimitForNetworkLayer() {
		return true
	}

	return sg.IsAddrIn(td.Addr)

}

func (sg *RouteSet) IsTransportProtocolAllowed(p uint16) bool {
	return sg.AllowedTransportLayerProtocols&p > 0
}

func (sg *RouteSet) IsAddrNetworkAllowed(a Addr) bool {

	if a.Network == "" {
		return sg.IsTransportProtocolAllowed(TCP)
	}

	p := StrToTransportProtocol(a.Network)

	return sg.IsTransportProtocolAllowed(p)
}

func (sg *RouteSet) IsUDPAllowed() bool {
	return sg.IsTransportProtocolAllowed(UDP)
}

func (sg *RouteSet) IsTCPAllowed() bool {
	return sg.IsTransportProtocolAllowed(TCP)
}

func (sg *RouteSet) IsNoLimitForNetworkLayer() bool {
	if (sg.NetRanger == nil || sg.NetRanger.Len() == 0) && len(sg.IPs) == 0 && len(sg.Match) == 0 && len(sg.Domains) == 0 && len(sg.Full) == 0 && len(sg.Countries) == 0 && len(sg.Geosites) == 0 {
		//如果仅限制了一个传输层协议，且本集合里没有任何其它内容，那就直接通过
		return true
	}
	return false
}

func (sg *RouteSet) IsAddrIn(a Addr) bool {
	//我们先过滤传输层，再过滤网络层, 因为传输层过滤非常简单。

	if !sg.IsAddrNetworkAllowed(a) {
		return false
	}

	//开始网络层判断
	if len(a.IP) > 0 {
		if sg.NetRanger != nil && sg.NetRanger.Len() > 0 {
			if has, _ := sg.NetRanger.Contains(a.IP); has {
				return true
			}
		}
		if len(sg.Countries) > 0 {

			if isoStr := GetIP_ISO(a.IP); isoStr != "" {
				if _, found := sg.Countries[isoStr]; found {
					return true
				}
			}

		}
		if len(sg.IPs) > 0 {
			if _, found := sg.IPs[a.GetNetIPAddr()]; found {
				return true
			}
		}
	}

	if a.Name != "" {

		if len(sg.Full) > 0 {
			if _, found := sg.Full[a.Name]; found {
				return true
			}
		}

		if len(sg.Domains) > 0 {

			if HasFullOrSubDomain(a.Name, MapDomainHaser(sg.Domains)) {
				return true
			}

		}

		if len(sg.Match) > 0 {
			for _, m := range sg.Match {
				if strings.Contains(a.Name, m) {
					return true
				}
			}
		}

		if len(sg.Regex) > 0 {
			for _, reg := range sg.Regex {
				if reg.MatchString(a.Name) {
					return true
				}
			}
		}

		if len(sg.Geosites) > 0 && len(GeositeListMap) > 0 {

			for _, g := range sg.Geosites {
				if IsDomainInsideGeosite(g, a.Name) {
					return true
				}
			}

		}

	}
	return false
}

//一个完整的 所有RouteSet的列表，进行路由时，直接遍历即可。
// 所谓的路由实际上就是分流。
type RoutePolicy struct {
	List []*RouteSet
}

func NewRoutePolicy() *RoutePolicy {
	return &RoutePolicy{
		List: make([]*RouteSet, 0, 2),
	}
}

func (rp *RoutePolicy) AddRouteSet(rs *RouteSet) {
	if rs != nil {
		rp.List = append(rp.List, rs)
	}
}

// 返回一个 proxy.Client 的 tag。
// 默认情况下，始终具有direct这个tag以及 proxy这个tag，无需用户额外在配置文件中指定。
// 默认如果不匹配任何值的话，就会流向 "proxy" tag，也就是客户设置的 remoteClient的值。
func (rp *RoutePolicy) GetOutTag(td *TargetDescription) string {
	for _, s := range rp.List {
		if s.IsIn(td) {
			switch n := len(s.OutTags); n {
			case 0:
				return s.OutTag
			case 1:
				return s.OutTags[0]
			default:
				return s.OutTags[rand.Intn(n)]
			}

		}
	}
	return "proxy"
}
