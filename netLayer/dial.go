package netLayer

import (
	"crypto/tls"
	"net"
	"syscall"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	//你可以通过向这个map插入 自定义函数的方式 来拓展 vs的 拨号功能, 可以拨号 其它 net包无法拨号的 network
	CustomDialerMap = make(map[string]func(address string, timeout time.Duration) (net.Conn, error))
)

//Dial 可以拨号tcp、udp、unix domain socket、tls 这几种协议。
//如果不是这几种之一，则会尝试查询 CustomDialerMap 找出匹配的函数进行拨号。
//如果找不到，则会使用net包的方法进行拨号（其会返回错误）。
func (a *Addr) Dial() (net.Conn, error) {
	var istls bool
	var resultConn net.Conn
	var err error

	switch n := a.Network; n {
	case "":
		a.Network = "tcp"
		goto tcp
	case "tcp", "tcp4", "tcp6":
		goto tcp
	case "tls": //此形式目前被用于dns配置中 的 dns over tls 的 url中
		istls = true
		goto tcp
	case "udp", "udp4", "udp6":
		ua := a.ToUDPAddr()

		if weKnowThatWeDontHaveIPV6 && a.IP.To4() == nil {
			return nil, ErrMachineCantConnectToIpv6
		}

		return DialUDP(ua)
	default:
		if len(CustomDialerMap) > 0 {
			if f := CustomDialerMap[n]; f != nil {
				return f(a.String(), time.Second*15)
			}
		}

		goto defaultPart

	}

tcp:

	if a.IP != nil {

		var tcpConn *net.TCPConn

		tcpConn, err = net.DialTCP("tcp", nil, &net.TCPAddr{
			IP:   a.IP,
			Port: a.Port,
		})

		if err == nil {
			tcpConn.SetWriteBuffer(utils.MaxPacketLen) //有时不设置writebuffer时，会遇到 write: no buffer space available 错误, 在实现vmess的 ChunkMasking 时 遇到了该问题。

		}

		resultConn = tcpConn
		goto dialedPart

	}

defaultPart:
	if istls {
		resultConn, err = net.DialTimeout("tcp", a.String(), time.Second*15)

	} else {
		resultConn, err = net.DialTimeout(a.Network, a.String(), time.Second*15)

	}

dialedPart:
	if istls && err == nil {

		conf := &tls.Config{}

		if a.Name != "" {
			conf.ServerName = a.Name
		} else {
			conf.InsecureSkipVerify = true
		}

		tlsconn := tls.Client(resultConn, conf)
		err = tlsconn.Handshake()
		return tlsconn, err
	}
	return resultConn, err

}

func (a Addr) DialWithOpt(sockopt *Sockopt) (net.Conn, error) {

	dialer := &net.Dialer{
		Timeout: time.Second * 8, //作为对照，v2ray默认是16秒
	}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			SetSockOpt(int(fd), sockopt, a.IsUDP(), a.IsIpv6())

		})
	}

	return dialer.Dial(a.Network, a.String())

}
