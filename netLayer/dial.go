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

	//本以为直接用 DialTCP 可以加速拨号，结果发现go官方包内部依然还是把地址转换回字符串再拨号

	if a.IP != nil {
		if a.IP.To4() == nil {
			if weKnowThatWeDontHaveIPV6 {
				return nil, ErrMachineCantConnectToIpv6
			} else {

				tcpConn, err2 := net.DialTCP("tcp6", nil, &net.TCPAddr{
					IP:   a.IP,
					Port: a.Port,
				})

				tcpConn.SetWriteBuffer(utils.MaxPacketLen) //有时不设置writebuffer时，会遇到 write: no buffer space available 错误, 在实现vmess的 ChunkMasking 时 遇到了该问题。

				resultConn, err = tcpConn, err2
				goto dialedPart
			}
		} else {

			tcpConn, err2 := net.DialTCP("tcp4", nil, &net.TCPAddr{
				IP:   a.IP,
				Port: a.Port,
			})

			tcpConn.SetWriteBuffer(utils.MaxPacketLen)

			resultConn, err = tcpConn, err2

			goto dialedPart
		}

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
		Timeout: time.Second * 8, //v2ray默认16秒，是不是太长了？？
	}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			SetSockOpt(int(fd), sockopt, a.IsUDP(), a.IsIpv6())

		})
	}

	return dialer.Dial(a.Network, a.String())

}
