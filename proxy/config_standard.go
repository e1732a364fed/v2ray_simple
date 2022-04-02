package proxy

import (
	"io/ioutil"
	"log"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/yl2chen/cidranger"
)

// CommonConf 是标准配置中 Listen和Dial 都有的部分
//如果新协议有其他新项，可以放入 Extra.
type CommonConf struct {
	Tag      string   `toml:"tag"`      //可选
	Protocol string   `toml:"protocol"` //约定，如果一个Protocol尾缀去掉了's'后仍然是一个有效协议，则该协议使用了 tls。这种方法继承自 v2simple，适合极简模式
	Uuid     string   `toml:"uuid"`     //一个用户的唯一标识，建议使用uuid，但也不一定
	Host     string   `toml:"host"`     //ip 或域名. 若unix domain socket 则为文件路径
	IP       string   `toml:"ip"`       //给出Host后，该项可以省略; 既有Host又有ip的情况比较适合cdn
	Port     int      `toml:"port"`     //若Network不为 unix , 则port项必填
	Version  int      `toml:"ver"`      //可选
	TLS      bool     `toml:"tls"`      //可选. 如果不使用 's' 后缀法，则还可以配置这一项来更清晰第标明使用tls
	Insecure bool     `toml:"insecure"` //tls 是否安全
	Alpn     []string `toml:"alpn"`

	Network string `toml:"network"` //默认使用tcp, network可选值为 tcp, udp, unix;

	AdvancedLayer string `toml:"advancedLayer"` //可不填，或者为ws，或者为grpc

	Path string `toml:"path"` //grpc或ws 的path

	Extra map[string]interface{} `toml:"extra"` //用于包含任意其它数据.虽然本包自己定义的协议肯定都是已知的，但是如果其他人使用了本包的话，那就有可能添加一些 新协议 特定的数据.
}

func (cc *CommonConf) GetAddrStr() string {
	switch cc.Network {
	case "unix":
		return cc.Host

	default:
		if cc.Host != "" {

			return cc.Host + ":" + strconv.Itoa(cc.Port)
		} else {
			return cc.IP + ":" + strconv.Itoa(cc.Port)

		}

	}

}

//和 GetAddr的区别是，它优先使用ip，其次再使用host
func (cc *CommonConf) GetAddrStrForListenOrDial() string {
	switch cc.Network {
	case "unix":
		return cc.Host

	default:
		if cc.IP != "" {
			return cc.IP + ":" + strconv.Itoa(cc.Port)

		} else {
			return cc.Host + ":" + strconv.Itoa(cc.Port)

		}

	}

}

// 监听所使用的设置, 使用者可被称为 listener 或者 inServer
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为监听地址与端口
type ListenConf struct {
	CommonConf
	Fallback any    `toml:"fallback"` //可选，默认回落的地址，一般可以是ip:port,数字port 或者 unix socket的文件名
	TLSCert  string `toml:"cert"`
	TLSKey   string `toml:"key"`

	//noroute 意味着 传入的数据 不会被分流，一定会被转发到默认的 dial
	// 这一项是针对 mycountry 分流功能的. 如果不设noroute, 且给定了 app.mycountry, 则所有listener 得到的流量都会被 试图 进行国别分流
	NoRoute bool `toml:"noroute"`

	TargetAddr string `toml:"target"` //若使用dokodemo协议，则这一项会给出. 格式 tcp://127.0.0.1:443 , 必须带scheme，以及端口。只能为tcp或udp

}

// 拨号所使用的设置, 使用者可被称为 dialer 或者 outClient
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为拨号地址与端口
type DialConf struct {
	CommonConf
	Utls bool `toml:"utls"`
}

type AppConf struct {
	LogLevel          *int   `toml:"loglevel"` //需要为指针, 否则无法判断0到底是未给出的默认值还是 显式声明的0
	DefaultUUID       string `toml:"default_uuid"`
	MyCountryISO_3166 string `toml:"mycountry" json:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流

	NoReadV bool `toml:"noreadv"`

	AdminPass string `toml:"admin_pass"`
}

type DnsConf struct {
	Hosts   map[string]any `toml:"hosts"`   //用于强制指定哪些域名会被解析为哪些具体的ip；可以为一个ip字符串，或者一个 []string 数组, 数组内可以是A,AAAA或CNAME
	Servers []any          `toml:"servers"` //可以为一个地址url字符串，或者为 SpecialDnsServerConf; 如果第一个元素是字符串形式，则此第一个元素将会被用作默认dns服务器
}

type SpecialDnsServerConf struct {
	Addr    string   `toml:"addr"`    //必须为 udp://1.1.1.1:53 这种格式
	Domains []string `toml:"domains"` //指定哪些域名需要通过 该dns服务器进行查询
}

type RuleConf struct {
	DialTag string `toml:"dialTag"`

	InTags []string `toml:"inTag"`

	Countries []string `toml:"country"` // 如果类似 !CN, 则意味着专门匹配不为CN 的国家（目前还未实现）
	IPs       []string `toml:"ip"`
	Domains   []string `toml:"domain"`
	Network   []string `toml:"network"`
}

//标准配置。默认使用toml格式
// toml：https://toml.io/cn/
// english: https://toml.io/en/
type Standard struct {
	App     *AppConf `toml:"app"`
	DnsConf *DnsConf `toml:"dns"`

	Listen []*ListenConf `toml:"listen"`
	Dial   []*DialConf   `toml:"dial"`

	Route     []*RuleConf               `toml:"route"`
	Fallbacks []*httpLayer.FallbackConf `toml:"fallback"`
}

func LoadTomlConfStr(str string) (c Standard, err error) {
	_, err = toml.Decode(str, &c)

	return
}

func LoadTomlConfFile(fileNamePath string) (Standard, error) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		return LoadTomlConfStr(string(bs))
	} else {
		return Standard{}, utils.ErrInErr{ErrDesc: "can't open config file", ErrDetail: err}
	}

}

func LoadRulesForRoutePolicy(rules []*RuleConf, policy *netLayer.RoutePolicy) {
	for _, rc := range rules {
		newrs := LoadRuleForRouteSet(rc)
		policy.List = append(policy.List, newrs)
	}
}

func LoadRuleForRouteSet(rule *RuleConf) (rs *netLayer.RouteSet) {
	rs = netLayer.NewFullRouteSet()
	rs.OutTag = rule.DialTag

	for _, c := range rule.Countries {
		rs.Countries[c] = true
	}

	for _, c := range rule.Domains {
		rs.Domains[c] = true
	}

	for _, c := range rule.InTags {
		rs.InTags[c] = true
	}

	//ip 过滤 需要 分辨 cidr 和普通ip

	for _, ipStr := range rule.IPs {
		if strings.Contains(ipStr, "/") {
			if _, net, err := net.ParseCIDR(ipStr); err == nil {
				rs.NetRanger.Insert(cidranger.NewBasicRangerEntry(*net))
			}
			continue
		}

		na, e := netip.ParseAddr(ipStr)
		if e == nil {
			rs.IPs[na] = true
		}
	}

	for _, ns := range rule.Network {
		tp := netLayer.StrToTransportProtocol(ns)
		rs.AllowedTransportLayerProtocols |= tp
	}

	return rs
}

func LoadDnsMachine(conf *DnsConf) *netLayer.DNSMachine {
	var dm = &netLayer.DNSMachine{}

	var ok = false

	if len(conf.Servers) > 0 {
		ok = true
		ss := conf.Servers
		first := ss[0]
		firstDealed := false

		switch value := first.(type) {
		case string:
			ad, e := netLayer.NewAddrByURL(value)
			if e != nil {
				log.Fatalf("LoadDnsMachine loading server err %s\n", e)

			}
			dm = netLayer.NewDnsMachine(&ad)
			firstDealed = true
		}

		if firstDealed {
			ss = ss[1:]
		}

		dm.SpecialServerPollicy = make(map[string]string)

		for _, s := range ss {
			switch value := s.(type) {
			case SpecialDnsServerConf:

				for _, d := range value.Domains {
					dm.SpecialServerPollicy[d] = value.Addr
				}

			}
		}

	}
	if conf.Hosts != nil {
		ok = true
		dm.SpecialIPPollicy = make(map[string][]netip.Addr)

		for thishost, things := range conf.Hosts {

			switch value := things.(type) {
			case string:
				ip := net.ParseIP(value)

				ad, _ := netip.AddrFromSlice(ip)

				dm.SpecialIPPollicy[thishost] = []netip.Addr{ad}

			case []string:
				for _, str := range value {
					ad, err := netLayer.NewAddrFromAny(str)
					if err != nil {
						log.Fatalf("LoadDnsMachine loading host err %s\n", err)
					}

					dm.SpecialIPPollicy[thishost] = append(dm.SpecialIPPollicy[thishost], ad.GetHashable().Addr())
				}
			}

		}
	}

	if !ok {
		return nil
	}
	return dm
}
