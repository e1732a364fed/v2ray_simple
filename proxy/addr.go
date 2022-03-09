package proxy

import (
	"net"
	"strconv"
)

// Addr represents a address that you want to access by proxy. Either Name or IP is used exclusively.
type Addr struct {
	Name  string // domain name
	IP    net.IP
	Port  int
	IsUDP bool
}

func NewAddrFromUDPAddr(addr *net.UDPAddr) *Addr {
	return &Addr{
		IP:    addr.IP,
		Port:  addr.Port,
		IsUDP: true,
	}
}

func NewAddr(addrStr string) (*Addr, error) {
	host, portStr, err := net.SplitHostPort(addrStr)
	if err != nil {
		return nil, err
	}
	if host == "" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(portStr)

	a := &Addr{Port: port}
	if ip := net.ParseIP(host); ip != nil {
		a.IP = ip
	} else {
		a.Name = host
	}
	return a, nil
}

// Return host:port string
func (a *Addr) String() string {
	port := strconv.Itoa(a.Port)
	if a.IP == nil {
		return net.JoinHostPort(a.Name, port)
	}
	return net.JoinHostPort(a.IP.String(), port)
}

func (a *Addr) UrlString() string {
	str := a.String()
	if a.IsUDP {
		return "udp://" + str
	}
	return "tcp://" + str
}

func (a *Addr) ToUDPAddr() *net.UDPAddr {
	if !a.IsUDP {
		return nil
	}
	ua, err := net.ResolveUDPAddr("udp", a.String())
	if err != nil {
		return nil
	}
	return ua
}

// Returned host string
func (a *Addr) HostStr() string {
	if a.IP == nil {
		return a.Name
	}
	return a.IP.String()
}

func (addr *Addr) Dial() (net.Conn, error) {
	if addr.IsUDP {
		return net.Dial("udp", addr.String())
	}
	return net.Dial("tcp", addr.String())
}

// Returned address bytes and type
func (a *Addr) AddressBytes() ([]byte, byte) {
	var addr []byte
	var atyp byte

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

	return addr, atyp
}

// ParseAddr parses the address in string s
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
