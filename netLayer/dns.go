package netLayer

import (
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

var globalDnsQueryMutex sync.Mutex

//domain必须是 dns.Fqdn 函数 包过的, 本函数不检查是否包过。如果不包过就传入，会报错。
// dns_type 为 miekg/dns 包中定义的类型, 如 TypeA, TypeAAAA, TypeCNAME.
// conn是一个建立好的 dns.Conn, 必须非空, 本函数不检查.
// theMux是与 conn相匹配的mutex, 这是为了防止同时有多个请求导致无法对口；内部若判断为nil,会主动使用一个全局mux.
// recursionCount 使用者统一填0 即可，用于内部 遇到cname时进一步查询时防止无限递归.
func DNSQuery(domain string, dns_type uint16, conn *dns.Conn, theMux *sync.Mutex, recursionCount int) net.IP {
	m := new(dns.Msg)
	m.SetQuestion((domain), dns_type) //为了更快，不使用 dns.Fqdn, 请调用之前先确保ok
	c := new(dns.Client)

	if theMux == nil {
		theMux = &globalDnsQueryMutex
	}

	theMux.Lock()

	r, _, err := c.ExchangeWithConn(m, conn)

	theMux.Unlock()

	if r == nil {
		if ce := utils.CanLogErr("dns query read err"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

	if r.Rcode != dns.RcodeSuccess {
		if ce := utils.CanLogDebug("dns query code err"); ce != nil {
			//dns查不到的情况是很有可能的，所以还是放在debug日志里
			ce.Write(zap.Error(err), zap.Int("rcode", r.Rcode), zap.String("value", r.String()))
		}
		return nil
	}

	switch dns_type {
	case dns.TypeA:
		for _, a := range r.Answer {
			if aa, ok := a.(*dns.A); ok {
				return aa.A
			}
		}
	case dns.TypeAAAA:
		for _, a := range r.Answer {
			if aa, ok := a.(*dns.AAAA); ok {
				return aa.AAAA
			}
		}
	}

	//没A和4A那就查cname在不在

	for _, a := range r.Answer {
		if aa, ok := a.(*dns.CNAME); ok {
			if ce := utils.CanLogDebug("dns query got cname"); ce != nil {
				ce.Write(zap.String("query", domain), zap.String("target", aa.Target))
			}

			if recursionCount > 2 {
				//不准循环递归，否则就是bug；因为有可能两个域名cname相互指向对方，好坏
				if ce := utils.CanLogDebug("dns query got cname but recursionCount>2"); ce != nil {
					ce.Write(zap.String("query", domain), zap.String("cname", aa.Target))
				}
				return nil
			}
			return DNSQuery(dns.Fqdn(aa.Target), dns_type, conn, theMux, recursionCount+1)
		}
	}

	return nil
}

// 给 miekg/dns.Conn 加一个互斥锁, 可保证同一时间仅有一个请求发生
// 这样就不会造成并发时的混乱
type DnsConn struct {
	*dns.Conn
	mutex sync.Mutex
}

//dns machine维持与多个dns服务器的连接(最好是udp这种无状态的)，并可以发起dns请求。
// 会缓存dns记录; 该设施是一个状态机, 所以叫 DNSMachine
// SpecialIPPollicy 用于指定特殊的 域名-ip映射，这样遇到这种域名时，不经过dns查询，直接返回预设ip
// SpecialServerPollicy 用于为特殊的 域名指定特殊的 dns服务器，这样遇到这种域名时，会通过该特定服务器查询
type DNSMachine struct {
	DefaultConn DnsConn
	conns       map[string]*DnsConn
	cache       map[string]net.IP

	SpecialIPPollicy map[string][]netip.Addr

	SpecialServerPollicy map[string]string //domain -> dns server name

	mutex sync.RWMutex //读写 conns, cache, SpecialIPPollicy, SpecialServerPollicy 时所使用的 mutex

}

//并不初始化所有内部成员, 只是创建空结构并拨号，若为nil则号也不拨
func NewDnsMachine(defaultDnsServerAddr *Addr) *DNSMachine {
	var dm DNSMachine
	if defaultDnsServerAddr != nil {

		var conn net.Conn
		var err error

		//实测 miekg/dns 必须用 net.PacketConn，不过本作udp最新代码已经支持了.
		// 不过dns还是没必要额外包装一次, 直接用原始的udp即可.

		if defaultDnsServerAddr.IsUDP() {
			conn, err = net.DialUDP("udp", nil, defaultDnsServerAddr.ToUDPAddr())
		} else {
			conn, err = defaultDnsServerAddr.Dial()

		}
		if err != nil {
			if ce := utils.CanLogErr("NewDnsMachine"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}

		dc := new(dns.Conn)
		dc.Conn = conn
		dm.DefaultConn.Conn = dc
	}

	return &dm
}

func (dm *DNSMachine) SetDefaultConn(c net.Conn) {
	dm.DefaultConn.Conn = new(dns.Conn)
	dm.DefaultConn.Conn.Conn = c
}

// 添加一个 特定名称的 域名服务器的 连接。
//name为该dns服务器的名称
func (dm *DNSMachine) AddConnForServer(name string, c net.Conn) {
	dc := new(dns.Conn)
	dc.Conn = c
	if dm.conns == nil {
		dm.conns = map[string]*DnsConn{}
	}
	dcc := &DnsConn{Conn: dc}
	dm.conns[name] = dcc
}

//传入的domain必须是不带尾缀点号的domain, 即没有包过 Fqdn
func (dm *DNSMachine) Query(domain string, dns_type uint16) (ip net.IP) {
	var generalCacheHit bool // 若读到了 cache 或 SpecialIPPollicy 的项, 则 generalCacheHit 为 true
	defer func() {
		if generalCacheHit {
			return
		}

		if len(ip) > 0 {
			if ce := utils.CanLogDebug("will add to dns cache"); ce != nil {
				ce.Write(zap.String("domain", domain))
			}

			dm.mutex.Lock()
			if dm.cache == nil {

				dm.cache = make(map[string]net.IP)
			}
			domain = strings.TrimSuffix(domain, ".")
			dm.cache[domain] = ip
			dm.mutex.Unlock()
		}
	}()

	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	//先从 cache找，有就直接返回
	//然后，
	//先查 specialIPPollicy，类似cache，有就直接返回
	//
	// 查不到再找 specialServerPollicy 看有没有特殊的dns服务器
	// 如果有指定服务器，用指定服务器查dns，若没有，用默认服务器查

	if dm.cache != nil {
		if ip = dm.cache[domain]; ip != nil {
			generalCacheHit = true
			if ce := utils.CanLogDebug("hit dns cache"); ce != nil {
				ce.Write(zap.String("domain", domain))
			}
			return
		}
	}

	if dm.SpecialIPPollicy != nil {
		if na := dm.SpecialIPPollicy[domain]; len(na) > 0 {

			switch dns_type {
			case dns.TypeA:
				for _, a := range na {

					if a.Is4() || a.Is4In6() {
						aa := a.As4()
						generalCacheHit = true
						return aa[:]
					}
				}
			case dns.TypeAAAA:
				for _, a := range na {
					if a.Is6() {
						aa := a.As16()
						generalCacheHit = true
						return aa[:]
					}
				}
			}

		}
	}

	theDNSServerConn := &dm.DefaultConn
	if dm.conns != nil && dm.SpecialServerPollicy != nil {

		if sn := dm.SpecialServerPollicy[domain]; sn != "" {

			if serConn := dm.conns[domain]; serConn != nil {
				theDNSServerConn = serConn
			}
		}
	}

	domain = dns.Fqdn(domain)
	return DNSQuery(domain, dns_type, theDNSServerConn.Conn, &theDNSServerConn.mutex, 0)
}
