// Package v2ray_v5 supports v2ray v5 config
// See https://www.v2fly.org/v5/config/
package v2ray_v5

type Conf struct {
	Log       *LogObject     `json:"log"`
	DNS       *DNSObject     `json:"dns"`
	Router    *RoutingObject `json:"router"`
	Inbounds  []Inbound      `json:"inbounds"`
	Outbounds []Outbound     `json:"outbounds"`
	Services  any            `json:"services"` //vs 不支持v2ray的 Services https://www.v2fly.org/v5/config/service.html
}

type LogObject struct {
	A LogSpecObject `json:"access"`
	E LogSpecObject `json:"error"`
}
type LogSpecObject struct {
	T string `json:"type"` //"None" | "Console" | "File"
	P string `json:"path"`
	L string `json:"level"` //"Debug" | "Info" | "Warning" | "Error" | "None"
}

type EndpointObject struct {
	A string `json:"address"`
	P int    `json:"port"`
}

type DNSObject struct {
	A        []NameServerObject  `json:"nameServer"`
	ClientIP string              `json:"clientIp"`      //当前网络的 IP 地址。用于 DNS 查询时通知 DNS 服务器，客户端所在的地理位置（不能是私有 IP 地址）。此功能需要 DNS 服务器支持 EDNS Client Subnet（RFC7871）。
	QS       string              `json:"queryStrategy"` //"UseIP" | "UseIPv4" | "UseIPv6"
	T        string              `json:"tag"`
	SH       []HostMappingObject `json:"staticHosts"`
	DC       bool                `json:"disableCache"`
	DF       bool                `json:"disableFallback"`
	DFIM     bool                `json:"disableFallbackIfMatch"`
}

type NameServerObject struct {
	Address      *EndpointObject        `json:"address"`
	ClientIP     string                 `json:"clientIp"`
	Port         uint16                 `json:"port"`
	SkipFallback bool                   `json:"skipFallback"`
	Domains      []PriorityDomainObject `json:"prioritizedDomain"`
	ExpectIPs    []GeoIP                `json:"expectIps"`
}

type PriorityDomainObject struct {
	T string `json:"type"` // "full" | "subdomain" | "keyword" | "regex"

	/*
			与 type 所对应的 domain 值。以下为 type 与domain 的对应关系：

		    full：当此域名完整匹配目标域名时，该规则生效。例如 v2ray.com 匹配 v2ray.com 但不匹配 www.v2ray.com。
		    regex：当 domain 所表示的正则表达式匹配目标域名时，该规则生效。例如 \.goo.*\.com$ 匹配 www.google.com、fonts.googleapis.com，但不匹配 google.com。
		    subdomain (推荐)：当此域名是目标域名或其子域名时，该规则生效。例如 v2ray.com 匹配 www.v2ray.com、v2ray.com，但不匹配 xv2ray.com。
		    keyword：当此字符串匹配目标域名中任意部分，该规则生效。比如 sina.com 可以匹配 sina.com、sina.com.cn、www.sina.com 和 www.sina.company，但不匹配 sina.cn。
	*/
	D string `json:"domain"`
}

type HostMappingObject struct {
	T string   `json:"type"`          // "full" | "subdomain" | "keyword" | "regex"
	D string   `json:"domain"`        //与 type 所对应的 domain 值。格式与 PriorityDomainObject 相同。
	P string   `json:"proxiedDomain"` //如指定 proxiedDomain，匹配的域名将直接使用该域名的查询结果，类似于 CNAME。
	I []string `json:"ip"`            //匹配的域名所映射的 IP 地址列表。
}

type GeoIP struct {
	Code     string     `json:"code"`
	FilePath string     `json:"filepath"`
	IM       bool       `json:"invertMatch"`
	CIDR     CIDRObject `json:"cidr"`
}

type CIDRObject struct {
	IA string `json:"ipAddr"`
	P  int    `json:"prefix"`
}

type RoutingObject struct {
	S  string     `json:"domainStrategy"` //AsIs | UseIp | IpIfNonMatch | IpOnDemand
	R  RuleObject `json:"rule"`
	BR any        `json:"balancingRule"`
}
type RuleObject struct {
	Code     string       `json:"tag"`
	FilePath string       `json:"balancingTag"`
	D        DomainObject `json:"domain"`
	GD       GeoDomain    `json:"geoDomain"`
	GI       GeoIP        `json:"geoip"`
	SGI      GeoIP        `json:"sourceGeoip"`

	/*
	   a-b：a 和 b 均为正整数，且小于 65536。这个范围是一个前后闭合区间，当端口落在此范围内时，此规则生效。
	   a：a 为正整数，且小于 65536。当目标端口为 a 时，此规则生效。
	   以上两种形式的混合，以逗号 "," 分隔。形如：53,443,1000-2000。
	*/
	PL  string   `json:"portList"`
	SPL string   `json:"sourcePortList"`
	P   []string `json:"protocol"` //[ "http" | "tls" | "bittorrent" ]
	UE  []string `json:"userEmail"`
	IT  []string `json:"inboundTag"`
	DM  string   `json:"domainMatcher"` //"linear" | "mph"
}

type DomainObject struct {
	T string `json:"type"` // "Plain" | "Regex" | "RootDomain" | "Full"
	V string `json:"value"`
}
type GeoDomain struct {
	P string `json:"filePath"`
	D string `json:"domain"`
	C string `json:"code"`
}
