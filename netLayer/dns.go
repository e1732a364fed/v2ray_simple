package netLayer

import (
	"errors"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/miekg/dns"
	"go.uber.org/zap"
)

var globalDnsQueryMutex sync.Mutex

var ErrRecursion = errors.New("multiple recursion not allowed")

// 判断 DNSQuery 返回的错误 是否是 Read底层连接 的错误
func Is_DNSQuery_returnType_ReadErr(err error) bool {
	if err == nil {
		return false
	}
	switch err {
	case os.ErrNotExist, dns.ErrRcode, ErrRecursion:
		return false
	default:
		return true
	}
}

// 筛除掉 Is_DNSQuery_returnType_ReadErr 时，err 为 net.Error.Timeout() 的情况
func Is_DNSQuery_returnType_ReadFatalErr(err error) bool {
	if !Is_DNSQuery_returnType_ReadErr(err) {
		return false
	}

	if ne, ok := err.(net.Error); ok {
		return !ne.Timeout()

	}

	return false
}

// domain必须是 dns.Fqdn 函数 包过的, 本函数不检查是否包过。如果不包过就传入，会报错。
// dns_type 为 miekg/dns 包中定义的类型, 目前只实现了 TypeA, TypeAAAA, TypeCNAME.
//
// conn是一个建立好的 dns.Conn, 必须非空, 本函数不检查.
// theMux是与 conn相匹配的mutex, 这是为了防止同时有多个请求导致无法对口；内部若判断为nil,会主动使用一个全局mux.
// recursionCount 使用者统一填0 即可，用于内部 遇到cname时进一步查询时防止无限递归.
//
// 如果从conn中Read后成功返回, 则可能返回如下几种错误 os.ErrNotExist (表示查无此记录), dns.ErrRcode (表示dns返回的 Rcode 不是 dns.RcodeSuccess), ErrRecursion,
// 如果不是这三个error, 那就是 从 该 conn 读取数据时出错了.
func DNSQuery(domain string, dns_type uint16, conn *dns.Conn, theMux *sync.Mutex, recursionCount int) (ip net.IP, ttl uint32, err error) {
	m := new(dns.Msg)
	m.SetQuestion((domain), dns_type) //为了更快，不使用 dns.Fqdn, 请调用之前先确保ok
	c := new(dns.Client)

	if theMux == nil {
		theMux = &globalDnsQueryMutex
	}

	theMux.Lock()

	var r *dns.Msg
	r, _, err = c.ExchangeWithConn(m, conn)

	theMux.Unlock()

	if r == nil {
		if ce := utils.CanLogErr("dns query read err"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if r.Rcode != dns.RcodeSuccess {
		if ce := utils.CanLogDebug("dns query code err"); ce != nil {
			//dns查不到的情况是很有可能的，所以还是放在debug日志里
			ce.Write(zap.Error(err), zap.Int("rcode", r.Rcode), zap.String("value", r.String()))
		}
		err = dns.ErrRcode
		return
	}

	switch dns_type {
	case dns.TypeA:
		for _, a := range r.Answer {
			if aa, ok := a.(*dns.A); ok {
				ip = aa.A
				ttl = aa.Hdr.Ttl
				return
			}
		}
	case dns.TypeAAAA:
		for _, a := range r.Answer {
			if aa, ok := a.(*dns.AAAA); ok {
				ip = aa.AAAA
				ttl = aa.Hdr.Ttl
				return
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
				err = ErrRecursion
				return
			}
			return DNSQuery(dns.Fqdn(aa.Target), dns_type, conn, theMux, recursionCount+1)
		}
	}

	err = os.ErrNotExist
	return
}

type DnsConn struct {
	*dns.Conn
	Name  string //我们这里惯例，直接使用配置文件中配置的url字符串作为Name
	raddr *Addr  //这个用于在Conn出故障后, 重新拨号时所使用

	// 加一个互斥锁, 可保证同一时间仅有一个 对 dns.Conn 的使用。
	// 这样就不会造成并发时的混乱
	mutex sync.Mutex

	garbageMark bool
}

type IPRecord struct {
	IP         net.IP
	TTL        uint32 //seconds
	RecordTime time.Time
}

// dns machine维持与多个dns服务器的连接(最好是udp这种无状态的)，并可以发起dns请求。
// 会缓存dns记录; 该设施是一个状态机, 所以叫 DNSMachine。
// SpecialIPPollicy 用于指定特殊的 域名-ip 映射，这样遇到这种域名时，不经过dns查询，直接返回预设ip。
// SpecialServerPollicy 用于为特殊的 域名指定特殊的 dns服务器，这样遇到这种域名时，会通过该特定服务器查询。
type DNSMachine struct {
	TypeStrategy int64  // 0, 4, 6, 40, 60
	TTLStrategy  uint32 // 0, 1, arbitrary，见 DnsConf 中的定义

	defaultConn DnsConn
	conns       map[string]*DnsConn
	cache       map[string]IPRecord //cache的key统一为 未经 Fqdn包装过的域名. 即尾部没有点号

	SpecialIPPollicy map[string][]netip.Addr

	SpecialServerPolicy map[string]string //domain -> dns server name

	mutex sync.RWMutex //读写 conns, cache, SpecialIPPollicy, SpecialServerPollicy 时所使用的 mutex

	listening bool
	listenUrl string
	server    *dns.Server
}

// Dial通过 c 内部设置好的地址进行拨号,并将 c.Conn.Conn 设为 新建立好的连接
func (c *DnsConn) Dial() error {
	nc, err := DialDnsAddr(c.raddr)
	if err != nil {
		return err
	}
	c.Conn.Conn = nc
	return nil
}

// 建立一个与dns服务器连接, 可为纯udp dns or DoT. if DoT, 则要求 addr.Network == "tls",
// 如果是纯udp的，要求 addr.IsUDP() == true
func DialDnsAddr(addr *Addr) (conn net.Conn, err error) {

	//实测 miekg/dns 要求传入的net.Conn必须用 net.PacketConn, 本作udp拨号所获的的对象已经支持了net.PacketConn接口.
	// 不过dns还是没必要额外包装一次, 直接用原始的udp即可.

	//在 miekg/dns 遇到非 net.PacketConn 的连接时，会采用不同的办法，先从数据读取一个长度信息，然后再读其它信息，可能它没有料到 net.Conn 被包装的情况, 所以我们需要额外处理一下。

	/*
		dns over tls rfc：https://datatracker.ietf.org/doc/html/rfc7858
		853端口

		根据
		https://datatracker.ietf.org/doc/html/rfc7858#section-3.3

		每个信息之前都要传2字节的信息长度

		所以显然 miekg/dns 认为传入的conn不是 net.UDPConn 就是 tls.Conn

		另外，miekg/dns 不支持 doh, 证据在 https://github.com/miekg/dns/pull/800

		就是因为 doh完全和 dot不同，使用了不同的数据结构.
	*/

	if addr.IsUDP() {
		conn, err = net.DialUDP("udp", nil, addr.ToUDPAddr())
	} else {
		conn, err = addr.Dial(nil, nil)

	}
	//todo: 以后支持DoH的话，要分离出https这个Network然后单独使用独特方法进行dial

	return
}

func (dm *DNSMachine) SetDefaultConn(c net.Conn, addr *Addr) {
	dm.defaultConn.Conn = new(dns.Conn)
	dm.defaultConn.Conn.Conn = c
	dm.defaultConn.raddr = addr
}

// 添加一个 特定的DNS服务器 , name为该dns服务器的名称. 若第一次调用, 则会设为 dm.DefaultConn
func (dm *DNSMachine) AddNewServer(name string, addr *Addr) error {

	if dm.defaultConn.Conn == nil { //若未配置过 DefaultConn
		dm.defaultConn = DnsConn{Conn: new(dns.Conn), raddr: addr, Name: name}
		err := dm.defaultConn.Dial()
		if err != nil {
			dm.defaultConn.Conn = nil
			return err
		}
	} else {

		dcc := &DnsConn{Conn: new(dns.Conn), raddr: addr, Name: name}
		err := dcc.Dial()
		if err != nil {
			return err
		}

		if dm.conns == nil {
			dm.conns = make(map[string]*DnsConn)
		}
		dm.conns[name] = dcc
	}

	return nil
}

func (dm *DNSMachine) Query(domain string) (ip net.IP) {
	switch dm.TypeStrategy {
	default:
		fallthrough
	case 0, 4:
		ip, _ = dm.QueryType(domain, dns.TypeA)
		if ip == nil {
			ip, _ = dm.QueryType(domain, dns.TypeAAAA)
		}
	case 6:
		ip, _ = dm.QueryType(domain, dns.TypeAAAA)
		if ip == nil {
			ip, _ = dm.QueryType(domain, dns.TypeA)
		}
	case 40:
		ip, _ = dm.QueryType(domain, dns.TypeA)
	case 60:
		ip, _ = dm.QueryType(domain, dns.TypeAAAA)
	}
	return
}

// 传入的domain必须是不带尾缀点号的domain, 即没有包过 Fqdn
func (dm *DNSMachine) QueryType(domain string, dns_type uint16) (ip net.IP, ttl uint32) {
	var generalCacheHit bool // 若读到了 cache 或 SpecialIPPollicy 的项, 则 generalCacheHit 为 true

	var theDNSServerConn *DnsConn
	// ttl  //用于cache

	defer func() {
		if theDNSServerConn != nil && theDNSServerConn.garbageMark {
			dm.mutex.Lock()
			delete(dm.conns, theDNSServerConn.Name)
			if theDNSServerConn == &dm.defaultConn {
				//如果DefaultConn都废了，那就糟糕
				//我们选一个备用的conn，升格为defaultConn

				dm.defaultConn.Conn = nil

				if len(dm.conns) > 0 {
					for name, c := range dm.conns {
						dm.defaultConn.Conn = c.Conn
						dm.defaultConn.garbageMark = false
						delete(dm.conns, name)
						break
					}
				}
				//没备用的，那就只好保持 dm.defaultConn.Conn 的 nil状态, 下一次dns查询就会失败

			}
			dm.mutex.Unlock()
		}

		if generalCacheHit {

			if ce := utils.CanLogDebug("[DNSMachine] hit cache"); ce != nil {
				ce.Write(zap.String("domain", domain), zap.String("ip", ip.String()))
			}
			return
		}

		if len(ip) > 0 {
			domain = strings.TrimSuffix(domain, ".")
			if ce := utils.CanLogDebug("[DNSMachine] will add to cache"); ce != nil {
				ce.Write(zap.String("domain", domain), zap.String("ip", ip.String()))
			}

			dm.mutex.Lock()
			if dm.cache == nil {

				dm.cache = make(map[string]IPRecord)
			}

			dm.cache[domain] = IPRecord{IP: ip, TTL: ttl, RecordTime: time.Now()}
			dm.mutex.Unlock()
		}
	}()

	// 查找步骤:
	//先从 cache找，有的话，若符合TTL策略，就直接返回；不符合策略或者找不到的话，进入下面步骤：
	//
	//查 specialIPPollicy，类似cache，有就直接返回
	//
	// 查不到再找 specialServerPolicy 看有没有特殊的dns服务器
	// 如果有指定服务器，用指定服务器查dns，若没有，用默认服务器查

	if dm.cache != nil {

		dm.mutex.RLock()
		ipRecord, ok := dm.cache[domain]
		dm.mutex.RUnlock()

		if ok {

			switch dm.TTLStrategy {
			case 0: // no timeout
			case 1: //strictly follow TTL
				now := time.Now()
				deadline := ipRecord.RecordTime.Add(time.Second * time.Duration(ipRecord.TTL))
				if now.After(deadline) {
					ok = false
				}
			default: //customized ttl
				now := time.Now()
				deadline := ipRecord.RecordTime.Add(time.Second * time.Duration(dm.TTLStrategy))
				if now.After(deadline) {
					ok = false
				}
			}

			if ok {
				ip = ipRecord.IP
				generalCacheHit = true

				return

			} else {
				dm.mutex.Lock()
				delete(dm.cache, domain)
				dm.mutex.Unlock()
			}
		}
	}

	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	if dm.SpecialIPPollicy != nil {
		if na := dm.SpecialIPPollicy[domain]; len(na) > 0 {

			switch dns_type {
			case dns.TypeA:
				for _, a := range na {

					if a.Is4() || a.Is4In6() {
						aa := a.As4()
						generalCacheHit = true
						return aa[:], uint32(dm.TTLStrategy)
					}
				}
			case dns.TypeAAAA:
				for _, a := range na {
					if a.Is6() {
						aa := a.As16()
						generalCacheHit = true
						return aa[:], uint32(dm.TTLStrategy)
					}
				}
			}

		}
	}

	theDNSServerConn = &dm.defaultConn
	if len(dm.conns) > 0 && len(dm.SpecialServerPolicy) > 0 {

		if dnsServerName := dm.SpecialServerPolicy[domain]; dnsServerName != "" {

			if serConn := dm.conns[dnsServerName]; serConn != nil {
				theDNSServerConn = serConn
			}
		}
	}

	if theDNSServerConn.Conn == nil { //如果配置文件只配置了自定义映射, 而没配置dns服务器的话, 那么我们就无法进行实际的dns查询; 或者配置了，但是因为Dial失败，导致没有 实际的Conn
		if ce := utils.CanLogDebug("[DNSMachine] no server configured, return nil."); ce != nil {
			ce.Write()
		}

		return
	}

	domain = dns.Fqdn(domain)

	if ce := utils.CanLogDebug("[DNSMachine] start querying"); ce != nil {
		ce.Write(zap.String("domain", domain), zap.String("through", theDNSServerConn.Name))
	}
	var err error

	ip, ttl, err = DNSQuery(domain, dns_type, theDNSServerConn.Conn, &theDNSServerConn.mutex, 0)

	if Is_DNSQuery_returnType_ReadFatalErr(err) {
		//如果是读取的、非timeout的错误，那么我们直接认为底层连接出故障了, 我们需要重新dial
		//因为 miekg/dns 包会设置4秒的timeout，所以确实要筛除timeout的情况

		theDNSServerConn.Conn.Close()
		err = theDNSServerConn.Dial()
		if err != nil {
			//再dial还是错误？那么就废了，
			if ce := utils.CanLogErr("[DNSMachine] Re-Dial Dns Server Failed"); ce != nil {
				ce.Write(zap.Error(err))
			}

			theDNSServerConn.garbageMark = true
		}

		//我们只是重新Dial，并不再次查询，否则就又递归了

	}
	return
}

// 使用通过配置设置好的监听地址进行监听
func (dm *DNSMachine) StartListen() {
	if dm.listenUrl == "" {
		return
	}
	e := dm.ListenUrl(dm.listenUrl)
	if e != nil {
		if ce := utils.CanLogErr("Failed in LoadDnsMachine, try listen failed "); ce != nil {
			ce.Write(zap.Error(e))
		}
	}

}

// 非阻塞, addr 为 url格式
func (dm *DNSMachine) ListenUrl(addr string) error {

	//测试: nslookup -port=8053 www.myfake.com  127.0.0.1

	a, e := NewAddrByURL(addr)
	network := a.Network

	if e != nil || network == "" {
		return utils.ErrInErr{ErrDesc: "dns listen url format wrong", ErrDetail: e, Data: addr}
	}
	if network == "tls" {
		network = "tcp-tls" //见 github.com/miekg/dns@v1.1.50/server.go 第315行
	}
	addr = a.String()

	server := &dns.Server{Addr: addr, Net: network, Handler: dm}

	if ce := utils.CanLogInfo("Start Dns server..."); ce != nil {
		ce.Write(zap.String("addr", addr))
	}

	go server.ListenAndServe()
	dm.server = server
	dm.listening = true

	return nil
}

// 如果调用过Listen，则Stop会关闭 dns监听
func (dm *DNSMachine) Stop() {
	if dm.listening {
		dm.listening = false

		if ce := utils.CanLogInfo("Stop Dns server..."); ce != nil {
			ce.Write()
		}

		dm.server.Shutdown()
		dm.server = nil
	}
}

// 实现 miekg/dns.Handler, 用于监听。不要直接调用该方法。
// 只查第一个question
func (dm *DNSMachine) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if r == nil || len(r.Question) == 0 {
		return
	}
	name := r.Question[0].Name
	qtype := r.Question[0].Qtype
	noDotName := strings.TrimSuffix(name, ".")

	if ce := utils.CanLogDebug("Dns got"); ce != nil {
		ce.Write(zap.String("name", noDotName), zap.Uint16("qtype", qtype))
	}

	ip, ttl := dm.QueryType(noDotName, qtype)

	if ce := utils.CanLogDebug("Dns ip for"); ce != nil {
		ce.Write(zap.String("name", noDotName), zap.String("ip", ip.String()))
	}

	// 构建返回信息
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	var dnsRR []dns.RR

	rr := new(dns.A)

	if dm.TTLStrategy != 1 {
		ttl = uint32(dm.TTLStrategy)
	}
	rr.Hdr = dns.RR_Header{Name: name, Rrtype: qtype, Class: dns.ClassINET, Ttl: ttl}
	rr.A = ip
	dnsRR = append(dnsRR, rr)

	m.Answer = dnsRR
	w.WriteMsg(m)
}
