package netLayer

import (
	"crypto/tls"
	"net"
	"syscall"
	"time"
)

func (addr *Addr) Dial() (net.Conn, error) {
	var istls bool
	var resultConn net.Conn
	var err error

	switch addr.Network {
	case "":
		addr.Network = "tcp"
		goto tcp
	case "tcp", "tcp4", "tcp6":
		goto tcp
	case "tls": //此形式目前被用于dns配置中 的 dns over tls 的 url中
		istls = true
		goto tcp
	case "udp", "udp4", "udp6":
		ua := addr.ToUDPAddr()

		if !machineCanConnectToIpv6 && addr.IP.To4() == nil {
			return nil, ErrMachineCantConnectToIpv6
		}

		return DialUDP(ua)
	default:

		goto defaultPart

	}

tcp:

	//本以为直接用 DialTCP 可以加速拨号，结果发现go官方包内部依然还是把地址转换回字符串再拨号

	if addr.IP != nil {
		if addr.IP.To4() == nil {
			if !machineCanConnectToIpv6 {
				return nil, ErrMachineCantConnectToIpv6
			} else {

				resultConn, err = net.DialTCP("tcp6", nil, &net.TCPAddr{
					IP:   addr.IP,
					Port: addr.Port,
				})
				goto dialedPart
			}
		} else {

			resultConn, err = net.DialTCP("tcp4", nil, &net.TCPAddr{
				IP:   addr.IP,
				Port: addr.Port,
			})
			goto dialedPart
		}

	}

defaultPart:
	resultConn, err = net.DialTimeout(addr.Network, addr.String(), time.Second*15)

dialedPart:
	if istls && err == nil {

		conf := &tls.Config{}

		if addr.Name != "" {
			conf.ServerName = addr.Name
		} else {
			conf.InsecureSkipVerify = true
		}

		tlsconn := tls.Client(resultConn, conf)
		err = tlsconn.Handshake()
		return tlsconn, err
	}
	return resultConn, err

}

func (addr Addr) DialWithOpt(sockopt *Sockopt) (net.Conn, error) {

	dialer := &net.Dialer{
		Timeout: time.Second * 8, //v2ray默认16秒，是不是太长了？？
	}
	dialer.Control = func(network, address string, c syscall.RawConn) error {
		return c.Control(func(fd uintptr) {
			SetSockOpt(int(fd), sockopt, addr.IsUDP(), addr.IsIpv6())

		})
	}

	return dialer.Dial(addr.Network, addr.String())

}
