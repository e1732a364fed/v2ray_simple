package netLayer

import (
	"errors"
	"math/rand"
	"net"
	"net/netip"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
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
	Name    string // domain name, or unix domain socket 的 文件路径
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

var (
	randPortBase int = 60000
)

func init() {
	if runtime.GOOS == "windows" {
		randPortBase = 45000 //windows在测试中发现高于五万的端口经常被占用
	}
}

//if mustValid is true, a valid port is assured.
// isudp is used to determine whether you want to use udp.
// depth 填0 即可，用于递归。
func RandPort(mustValid, isudp bool, depth int) (p int) {
	p = rand.Intn(randPortBase) + 4096
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

			if depth < 20 {
				return RandPort(mustValid, true, depth+1)

			} else {
				if ce := utils.CanLogDebug("Get RandPort udp but got err, and depth reach limit, return directly"); ce != nil {
					ce.Write()
				}
				return
			}

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

			if depth < 20 {
				return RandPort(mustValid, false, depth+1)

			} else {
				if ce := utils.CanLogDebug("Get RandPort udp but got err, and depth reach limit, return directly"); ce != nil {
					ce.Write()
				}
				return
			}
		}

	}

	return
}

//use a new seed each time called
func RandPortStr_safe(mustValid, isudp bool) string {
	rand.Seed(time.Now().Unix())
	return strconv.Itoa(RandPort(mustValid, isudp, 0))
}

func RandPortStr(mustValid, isudp bool) string {
	return strconv.Itoa(RandPort(mustValid, isudp, 0))
}

func RandPort_andStr(mustValid, isudp bool) (int, string) {
	pt := RandPort(mustValid, isudp, 0)
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
func NewAddrFromTCPAddr(addr *net.TCPAddr) Addr {
	return Addr{
		IP:      addr.IP,
		Port:    addr.Port,
		Network: "tcp",
	}
}

//addrStr格式一般为 host:port ；如果不含冒号，将直接认为该字符串是域名或文件名
func NewAddr(addrStr string) (Addr, error) {
	if !strings.Contains(addrStr, ":") {
		//unix domain socket, or 域名默认端口的情况
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
	if err != nil {
		return Addr{}, err
	}

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
	if err != nil {
		return Addr{}, err
	}

	a := Addr{Port: port}
	if ip := net.ParseIP(host); ip != nil {
		a.IP = ip
	} else {
		a.Name = host
	}

	a.Network = u.Scheme

	return a, nil
}

//会根据thing的类型 生成实际addr； 可以为数字端口, 或 类似tcp://ip:port的url, 或ip:port字符串，或一个 文件路径(unix domain socket), or *net.TCPAddr / *net.UDPAddr / net.Addr
func NewAddrFromAny(thing any) (addr Addr, err error) {
	var integer int
	var dest_type byte = 0 //0: port, 1: ip:port, 2: unix domain socket
	var dest_string string

	switch value := thing.(type) {
	case float64: //json 默认把数字转换成float64，就算是整数也一样

		if value > 65535 || value < 0 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}

		integer = int(value)
	case float32:
		if value > 65535 || value < 0 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}

		integer = int(value)
	case int64: //toml包 默认把整数转换成int64

		if value > 65535 || value < 0 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}
		integer = int(value)
	case int:
		if value > 65535 || value < 0 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}

		integer = value
	case int32:

		if value > 65535 || value < 0 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}
		integer = int(value)
	case int16:
		integer = int(value)
	case int8:
		integer = int(value)

	case uint64:

		if value > 65535 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}
		integer = int(value)
	case uint:

		if value > 65535 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}
		integer = int(value)
	case uint32:
		if value > 65535 {
			err = utils.ErrInErr{ErrDesc: "Invalid port", Data: value}
			return
		}
		integer = int(value)
	case uint16:
		integer = int(value)
	case uint8:
		integer = int(value)

	case string:
		//先判断是不是url
		addr, err = NewAddrByURL(value)
		if err == nil {
			return
		} else {
			err = nil
		}

		//不是url时，有两种情况, 带冒号的 ip:port, or unix domain socket 的文件路径

		if strings.Contains(value, ":") {
			dest_type = 1
			dest_string = value
		} else {
			//不带冒号这里就直接认为是 unix domain socket

			dest_type = 2
			dest_string = value
		}
	case *net.TCPAddr:
		return NewAddrFromTCPAddr(value), nil

	case *net.UDPAddr:
		return NewAddrFromUDPAddr(value), nil
	case net.Addr:
		var host, port string
		host, port, err = net.SplitHostPort(value.String())
		if err != nil {
			return
		}
		addr.Network = value.Network()
		addr.IP = net.ParseIP(host)
		addr.Port, err = strconv.Atoi(port)
		return

	default:
		err = utils.ErrInErr{ErrDesc: "Failed in Fallback dest config type", Data: reflect.TypeOf(thing)}
		return
	}

	switch dest_type {
	case 0: //只给出数字的情况; 此时我们认为 该数字为端口、 ip为本机。
		addr = Addr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: integer,
		}
	case 1:
		addr, err = NewAddrByHostPort(dest_string)
		if err != nil {
			err = utils.ErrInErr{ErrDesc: "Failed in addr create with given string", ErrDetail: err, Data: dest_string}
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
			copy(addr, ip)
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

//依照 vmess/vless 协议的格式 依次读取 地址的 port, 域名/ip 信息
func V2rayGetAddrFrom(buf utils.ByteReader) (addr Addr, err error) {

	pb1, err := buf.ReadByte()
	if err != nil {
		return
	}

	pb2, err := buf.ReadByte()
	if err != nil {
		return
	}

	port := uint16(pb1)<<8 + uint16(pb2)
	if port == 0 {
		err = utils.ErrInvalidData
		return
	}
	addr.Port = int(port)

	var b1 byte

	b1, err = buf.ReadByte()
	if err != nil {
		return
	}

	switch b1 {
	case AtypDomain:
		var b2 byte
		b2, err = buf.ReadByte()
		if err != nil {
			return
		}

		if b2 == 0 {
			err = errors.New("got ATypDomain with domain lenth marked as 0")
			return
		}

		bs := utils.GetBytes(int(b2))
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}

		if n != int(b2) {
			err = utils.ErrShortRead
			return
		}
		addr.Name = string(bs[:n])

	case AtypIP4:
		bs := make([]byte, 4)
		var n int
		n, err = buf.Read(bs)

		if err != nil {
			return
		}
		if n != 4 {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	case AtypIP6:
		bs := make([]byte, net.IPv6len)
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}
		if n != net.IPv6len {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	default:
		err = utils.ErrInvalidData
		return
	}

	return
}

const MixNetworkName = "tcp/udp"

type TCP_or_UDPAddr struct {
	*net.TCPAddr
	*net.UDPAddr
}

func (tu *TCP_or_UDPAddr) Network() string {
	return MixNetworkName
}
func (tu *TCP_or_UDPAddr) String() string {
	return tu.TCPAddr.String() + " / " + tu.UDPAddr.String()
}

func StrToNetAddr(network, s string) (net.Addr, error) {
	if network == "" {
		network = "tcp"
	}
	realNet := StrToTransportProtocol(network)
	switch realNet {
	case IP:
		return net.ResolveIPAddr(network, s)
	case TCP:
		if !strings.Contains(s, ":") {
			s += ":0"
		}
		return net.ResolveTCPAddr(network, s)

	case UDP:
		if !strings.Contains(s, ":") {
			s += ":0"
		}
		return net.ResolveUDPAddr(network, s)

	case Mix:
		ta, e := StrToNetAddr("tcp", s)
		if e != nil {
			return nil, e
		}

		ua, e := StrToNetAddr("udp", s)
		if e != nil {
			return nil, e
		}

		return &TCP_or_UDPAddr{TCPAddr: ta.(*net.TCPAddr), UDPAddr: ua.(*net.UDPAddr)}, nil

	case UNIX:
		return net.ResolveUnixAddr(network, s)
	default:
		return nil, utils.ErrWrongParameter
	}
}
