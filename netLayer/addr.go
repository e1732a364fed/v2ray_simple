package netLayer

import (
	"errors"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

// Atyp, for vless and vmess; 注意与 trojan和socks5的区别，trojan和socks5的相同含义的值是1，3，4
const (
	AtypIP4    byte = 1
	AtypDomain byte = 2
	AtypIP6    byte = 3
)

//默认netLayer的 AType (AtypIP4,AtypIP6,AtypDomain) 遵循v2ray标准的定义;
// 如果需要符合 socks5/trojan标准, 需要用本函数转换一下。
// 即从 123 转换到 134
func ATypeToSocks5Standard(atype byte) byte {
	if atype == 1 {
		return 1
	}
	return atype + 1
}

// Addr represents a address that you want to access by proxy. Either Name or IP is used exclusively.
// Addr完整地表示了一个 传输层的目标，同时用 Network 字段 来记录网络层协议名
// Addr 还可以用Dial 方法直接进行拨号
type Addr struct {
	Network string
	Name    string // domain name, 或者 unix domain socket 的 文件路径
	IP      net.IP
	Port    int
}

type HashableAddr struct {
	Network, Name string
	netip.AddrPort
}

type AddrData struct {
	Addr Addr
	Data []byte
}

//if mustValid is true, a valid port is assured.
// isudp is used to determine whether you want to use udp
func RandPort(mustValid, isudp bool) (p int) {
	p = rand.Intn(60000) + 4096
	if !mustValid {
		return
	}
	if isudp {
		listener, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: p,
		})

		if listener != nil {
			listener.Close()
		}

		if err != nil {
			if ce := utils.CanLogDebug("Get RandPort udp but got err, trying again"); ce != nil {
				ce.Write()
			}

			return RandPort(mustValid, true)
		}
	} else {
		listener, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: p,
		})

		if listener != nil {
			listener.Close()

		}
		if err != nil {
			if ce := utils.CanLogDebug("Get RandPort tcp but got err, trying again"); ce != nil {
				ce.Write()
			}

			return RandPort(mustValid, false)
		}

	}

	return
}

func RandPortStr(mustValid, isudp bool) string {
	return strconv.Itoa(RandPort(mustValid, isudp))
}

func RandPort_andStr(mustValid, isudp bool) (int, string) {
	pt := RandPort(mustValid, isudp)
	return pt, strconv.Itoa(pt)
}

func GetRandLocalAddr(mustValid, isudp bool) string {
	return "0.0.0.0:" + RandPortStr(mustValid, isudp)
}

func GetRandLocalPrivateAddr(mustValid, isudp bool) string {
	return "127.0.0.1:" + RandPortStr(mustValid, isudp)
}

func NewAddrFromUDPAddr(addr *net.UDPAddr) Addr {
	return Addr{
		IP:      addr.IP,
		Port:    addr.Port,
		Network: "udp",
	}
}

//addrStr格式一般为 host:port ；如果不含冒号，将直接认为该字符串是域名或文件名
func NewAddr(addrStr string) (Addr, error) {
	if !strings.Contains(addrStr, ":") {
		//unix domain socket, 或者域名默认端口的情况
		return Addr{Name: addrStr}, nil
	}

	return NewAddrByHostPort(addrStr)
}

//hostPortStr格式 必须为 host:port，本函数不对此检查
func NewAddrByHostPort(hostPortStr string) (Addr, error) {
	host, portStr, err := net.SplitHostPort(hostPortStr)
	if err != nil {
		return Addr{}, err
	}
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(portStr)

	a := Addr{Port: port}
	if ip := net.ParseIP(host); ip != nil {

		a.IP = ip
	} else {
		a.Name = host
	}
	return a, nil
}

// 如 tcp://127.0.0.1:443 , tcp://google.com:443 ;
// 不支持unix domain socket, 因为它是文件路径, / 还需要转义，太麻烦;不是我们代码麻烦, 而是怕用户嫌麻烦
func NewAddrByURL(addrStr string) (Addr, error) {

	u, err := url.Parse(addrStr)
	if err != nil {
		return Addr{}, err
	}
	if u.Scheme == "unix" {
		return Addr{}, errors.New("parse unix domain socket by url is not supported")
	}
	addrStr = u.Host

	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		return Addr{}, err
	}
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(portStr)

	a := Addr{Port: port}
	if ip := net.ParseIP(host); ip != nil {
		a.IP = ip
	} else {
		a.Name = host
	}

	a.Network = u.Scheme

	return a, nil
}

//会根据thing的类型 生成实际addr； 可以为数字端口，或者带冒号的字符串，或者一个 文件路径(unix domain socket)
func NewAddrFromAny(thing any) (addr Addr, err error) {
	var integer int
	var dest_type byte = 0 //0: port, 1: ip:port, 2: unix domain socket
	var dest_string string

	switch value := thing.(type) {
	case float64: //json 默认把数字转换成float64，就算是整数也一样

		if value > 65535 || value < 1 {
			err = utils.ErrInErr{ErrDesc: "int port not valid", Data: value}
			return
		}

		integer = int(value)

	case int64: //toml包 默认把整数转换成int64
		integer = int(value)
	case string:
		//先判断是不是url
		addr, err = NewAddrByURL(value)
		if err == nil {
			return
		} else {
			err = nil
		}

		//不是url时，有两种情况, 带冒号的 ip:port, 或者 unix domain socket 的文件路径

		if strings.Contains(value, ":") {
			dest_type = 1
			dest_string = value
		} else {
			//不带冒号这里就直接认为是 unix domain socket

			dest_type = 2
			dest_string = value
		}

	default:
		err = utils.ErrInErr{ErrDesc: "Fallback dest config type err", Data: reflect.TypeOf(thing)}
		return
	}

	switch dest_type {
	case 0: //只给出数字的情况, 认为该数字为端口, ip为本机。
		addr = Addr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: integer,
		}
	case 1:
		addr, err = NewAddrByHostPort(dest_string)
		if err != nil {
			err = utils.ErrInErr{ErrDesc: "addr create with given string failed", ErrDetail: err, Data: dest_string}
			return
		}
	case 2:
		addr = Addr{
			Network: "unix",
			Name:    dest_string,
		}
	}

	return
}

func (a *Addr) GetHashable() (ha HashableAddr) {
	theip := a.IP
	if i4 := a.IP.To4(); i4 != nil {
		theip = i4 //能转成ipv4则必须转，否则虽然是同一个ip，但是如果被表示成了ipv6的形式，相等比较还是会失败
	}
	ip, _ := netip.AddrFromSlice(theip)

	ha.AddrPort = netip.AddrPortFrom(ip, uint16(a.Port))
	ha.Network = a.Network
	ha.Name = a.Name
	return
}

// Return host:port string. 若网络为unix，直接返回 a.Name.
// 若有Name而没有ip，则返回 a.Name:a.Port . 否则返回 a.IP: a.Port;
func (a *Addr) String() string {
	if a.Network == "unix" {
		return a.Name
	} else {
		port := strconv.Itoa(a.Port)
		if a.IP == nil {
			return net.JoinHostPort(a.Name, port)
		}
		return net.JoinHostPort(a.IP.String(), port)
	}

}

//返回以url表示的 地址. unix的话文件名若带斜杠则会被转义
func (a *Addr) UrlString() string {
	if a.Network != "" {
		return a.Network + "://" + url.PathEscape(a.String())

	} else {
		return "tcp://" + a.String()

	}

}

func (a *Addr) IsEmpty() bool {
	return a.Name == "" && len(a.IP) == 0 && a.Network == "" && a.Port == 0
}

func (a *Addr) IsIpv6() bool {
	return a.IP.To4() == nil
}

func (a *Addr) GetNetIPAddr() (na netip.Addr) {
	if len(a.IP) < 1 {
		return
	}

	na, _ = netip.AddrFromSlice(a.IP)

	return
}

//a.Network == "udp", "udp4", "udp6"
func (a *Addr) IsUDP() bool {
	return IsStrUDP_network(a.Network)
}

//如果a里只含有域名，则会自动解析域名为IP。
func (a *Addr) ToUDPAddr() *net.UDPAddr {
	ua, err := net.ResolveUDPAddr("udp", a.String())
	if err != nil {
		return nil
	}
	return ua
}

func (a *Addr) ToTCPAddr() *net.TCPAddr {
	ta, err := net.ResolveTCPAddr("tcp", a.String())
	if err != nil {
		return nil
	}
	return ta
}

// Returned host string
func (a *Addr) HostStr() string {
	if a.IP == nil {
		return a.Name
	}
	return a.IP.String()
}

// 如果a的ip不为空，则会返回 AtypIP4 或 AtypIP6, 否则会返回 AtypDomain
// Return address bytes and type
// 如果atyp类型是 域名，则 第一字节为该域名的总长度, 其余字节为域名内容。
// 如果类型是ip，则会拷贝出该ip的数据的副本。
func (a *Addr) AddressBytes() (addr []byte, atyp byte) {

	if a.IP != nil {
		if ip4 := a.IP.To4(); ip4 != nil {
			addr = make([]byte, net.IPv4len)
			atyp = AtypIP4
			copy(addr[:], ip4)
		} else {
			addr = make([]byte, net.IPv6len)
			atyp = AtypIP6
			copy(addr[:], a.IP)
		}
	} else {
		if len(a.Name) > 255 {
			return nil, 0
		}
		addr = make([]byte, 1+len(a.Name))
		atyp = AtypDomain
		addr[0] = byte(len(a.Name))
		copy(addr[1:], a.Name)
	}

	return
}

// ParseAddr 分析字符串，并按照特定方式返回 地址类型 atyp,地址数据 addr []byte,以及端口号,
//   如果解析出的地址是ip，则 addr返回 net.IP;
//  如果解析出的地址是 域名，则第一字节为域名总长度, 剩余字节为域名内容
func ParseStrToAddr(s string) (atyp byte, addr []byte, port_uint16 uint16, err error) {

	var host string
	var portStr string

	host, portStr, err = net.SplitHostPort(s)
	if err != nil {
		return
	}

	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			addr = make([]byte, net.IPv4len)
			atyp = AtypIP4
			copy(addr[:], ip4)
		} else {
			addr = make([]byte, net.IPv6len)
			atyp = AtypIP6
			copy(addr[:], ip)
		}
	} else {
		if len(host) > 255 {
			return
		}
		addr = make([]byte, 1+len(host))
		atyp = AtypDomain
		addr[0] = byte(len(host))
		copy(addr[1:], host)
	}

	var portUint64 uint64
	portUint64, err = strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return
	}

	port_uint16 = uint16(portUint64)

	return
}

func UDPAddr_v4_to_Bytes(addr *net.UDPAddr) [6]byte {
	ip := addr.IP.To4()

	port := uint16(addr.Port)

	var allByte [6]byte
	abs := allByte[:]
	copy(abs, ip)
	copy(abs[4:], (*(*[2]byte)(unsafe.Pointer(&port)))[:])

	return allByte
}

func UDPAddr2AddrPort(ua *net.UDPAddr) netip.AddrPort {
	if ua == nil {
		return netip.AddrPort{}
	}
	a, _ := netip.AddrFromSlice(ua.IP)
	return netip.AddrPortFrom(a, uint16(ua.Port))
}
