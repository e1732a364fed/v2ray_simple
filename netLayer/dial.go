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

const (
	defaultDialTimeout = time.Second * 8 //作为对照，v2ray默认是16秒
)

//Dial 可以拨号tcp、udp、unix domain socket、tls 这几种协议。
//如果不是这几种之一，则会尝试查询 CustomDialerMap 找出匹配的函数进行拨号。
//如果找不到，则会使用net包的方法进行拨号（其会返回错误）。
func (a *Addr) Dial(sockopt *Sockopt) (net.Conn, error) {
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

		if sockopt == nil {

			return DialUDP(ua)
		} else {

			var c net.Conn
			c, err = a.DialWithOpt(sockopt)
			if err == nil {
				uc := c.(*net.UDPConn)
				return NewUDPConn(ua, uc, true), nil
			} else {
				return nil, err
			}

		}

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

		if sockopt == nil {
			tcpConn, err = net.DialTCP("tcp", nil, &net.TCPAddr{
				IP:   a.IP,
				Port: a.Port,
			})
		} else {
			var c net.Conn
			c, err = a.DialWithOpt(sockopt)
			if err == nil {
				tcpConn = c.(*net.TCPConn)
			}
		}

		if err == nil {
			tcpConn.SetWriteBuffer(utils.MaxPacketLen) //有时不设置writebuffer时，会遇到 write: no buffer space available 错误, 在实现vmess的 ChunkMasking 时 遇到了该问题。

		}

		resultConn = tcpConn
		goto dialedPart

	}

defaultPart:
	if istls {
		//若tls到达了这里，则说明a的ip没有给出，而只给出了域名，所以上面tcp部分没有直接拨号

		if sockopt == nil {
			resultConn, err = net.DialTimeout("tcp", a.String(), defaultDialTimeout)

		} else {
			newA := *a
			newA.Network = "tcp"
			resultConn, err = newA.DialWithOpt(sockopt)
		}

	} else {
		//一般情况下，unix domain socket 会到达这里，其他情况则都被前面代码捕获到了
		if sockopt == nil {
			resultConn, err = net.DialTimeout(a.Network, a.String(), defaultDialTimeout)
		} else {
			resultConn, err = a.DialWithOpt(sockopt)
		}

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
		Timeout: defaultDialTimeout,
	}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			SetSockOpt(int(fd), sockopt, a.IsUDP(), a.IsIpv6())

		})
	}

	return dialer.Dial(a.Network, a.String())

}
