package netLayer

import (
	"net"
	"net/netip"
	"sync"

	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func DNSQuery(domain string, dns_type uint16, conn *dns.Conn, recursionCount int) net.IP {
	m := new(dns.Msg)
	m.SetQuestion((domain), dns_type) //为了更快，不使用 dns.Fqdn, 请调用之前先确保ok
	c := new(dns.Client)

	r, _, err := c.ExchangeWithConn(m, conn)
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
				if ce := utils.CanLogDebug("dns query got cname bug recursionCount>2"); ce != nil {
					ce.Write(zap.String("query", domain), zap.String("target", aa.Target))
				}
				return nil
			}
			return DNSQuery(aa.Target, dns_type, conn, recursionCount+1)
		}
	}

	return nil
}

//dns machine维持与多个dns服务器的连接(最好是udp这种无状态的)，并可以发起dns请求
// 会缓存dns记录
type DNSMachine struct {
	DefaultConn *dns.Conn
	conns       map[string]*dns.Conn
	cache       map[string]net.IP

	SpecialIPPollicy map[string][]netip.Addr

	SpecialServerPollicy map[string]string //domain -> dns server name 的map

	mux sync.RWMutex
}

//并不初始化所有内部成员
func NewDnsMachine(defaultAddr *Addr) *DNSMachine {
	var dm DNSMachine
	if defaultAddr != nil {

		var conn net.Conn
		var err error

		if defaultAddr.IsUDP() {
			conn, err = net.DialUDP("udp", nil, defaultAddr.ToUDPAddr())
		} else {
			conn, err = defaultAddr.Dial()

		}
		if err != nil {
			if ce := utils.CanLogErr("NewDnsMachine"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}

		dc := new(dns.Conn)
		dc.Conn = conn
		dm.DefaultConn = dc
	}

	return &dm
}

func (dm *DNSMachine) SetDefaultConn(c net.Conn) {
	dm.DefaultConn = new(dns.Conn)
	dm.DefaultConn.Conn = c
}

//name为该dns服务器的名称
func (dm *DNSMachine) AddConnForServer(name string, c net.Conn) {
	dc := new(dns.Conn)
	dc.Conn = c
	if dm.conns == nil {
		dm.conns = make(map[string]*dns.Conn)
	}
	dm.conns[name] = dc
}

func (dm *DNSMachine) Query(domain string, dns_type uint16) (ip net.IP) {
	defer func() {
		if len(ip) > 0 {
			//log.Println("will add to cache", domain, ip)

			dm.mux.Lock()
			if dm.cache == nil {

				dm.cache = make(map[string]net.IP)
			}
			dm.cache[domain] = ip
			dm.mux.Unlock()
		}
	}()

	dm.mux.RLock()
	defer dm.mux.RUnlock()

	//先从 cache找，有就直接返回
	//然后，
	//先查 specialIPPollicy，类似cache，有就直接返回
	//
	// 查不到再找 specialServerPollicy 看有没有特殊的dns服务器
	// 如果有指定服务器，用指定服务器查dns，若没有，用默认服务器查

	if dm.cache != nil {
		if ip = dm.cache[domain]; ip != nil {
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
						return aa[:]
					}
				}
			case dns.TypeAAAA:
				for _, a := range na {
					if a.Is6() {
						aa := a.As16()
						return aa[:]
					}
				}
			}

		}
	}

	theDNSServerConn := dm.DefaultConn
	if dm.conns != nil && dm.SpecialServerPollicy != nil {

		if sn := dm.SpecialServerPollicy[domain]; sn != "" {

			if serConn := dm.conns[domain]; serConn != nil {
				theDNSServerConn = serConn
			}
		}
	}

	domain = dns.Fqdn(domain)
	return DNSQuery(domain, dns_type, theDNSServerConn, 0)
}
