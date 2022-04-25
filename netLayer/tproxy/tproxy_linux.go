/*
Package tproxy implements tproxy.

透明代理只能用于linux。

About TProxy 关于透明代理

透明代理原理
https://www.kernel.org/doc/html/latest/networking/tproxy.html

golang 示例
https://github.com/LiamHaworth/go-tproxy/blob/master/tproxy_tcp.go

c 语言 示例
https://github.com/FarFetchd/simple_tproxy_example/blob/master/tproxy_captive_portal.c


关键点在于

1. 要使用 syscall.IP_TRANSPARENT 监听

2. 监听到的 连接 的 localAddr实际上是 真实的目标地址, 而不是我们监听的地址;


我们在本包里要做的事情就是 模仿 上面的 golang示例,

但是，上面的go示例有一个特点, 它是直接利用客户端自己的地址+reuse端口的方法去拨号实际地址的,而我们不需要那样做。

而且, udp 的过程更加特殊。

总之，这种情况完全不适配 proxy.Server 的接口, 应该单独拿出来, 属于网络层的特殊情况.

另外就是，偶然发现，trojan-go也是使用的 上面的示例的代码。

同时，trojan-go还使用了
https://github.com/cybozu-go/transocks/blob/master/original_dst_linux.go

Iptables

iptables配置教程：
https://toutyrater.github.io/app/tproxy.html

下面把该教程的重要部分搬过来。


	ip rule add fwmark 1 table 100
	ip route add local 0.0.0.0/0 dev lo table 100

	iptables -t mangle -N V2RAY
	iptables -t mangle -A V2RAY -d 127.0.0.1/32 -j RETURN
	iptables -t mangle -A V2RAY -d 224.0.0.0/4 -j RETURN
	iptables -t mangle -A V2RAY -d 255.255.255.255/32 -j RETURN
	iptables -t mangle -A V2RAY -d 192.168.0.0/16 -p tcp -j RETURN
	iptables -t mangle -A V2RAY -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
	iptables -t mangle -A V2RAY -p udp -j TPROXY --on-port 12345 --tproxy-mark 1
	iptables -t mangle -A V2RAY -p tcp -j TPROXY --on-port 12345 --tproxy-mark 1
	iptables -t mangle -A PREROUTING -j V2RAY

	iptables -t mangle -N V2RAY_MASK
	iptables -t mangle -A V2RAY_MASK -d 224.0.0.0/4 -j RETURN
	iptables -t mangle -A V2RAY_MASK -d 255.255.255.255/32 -j RETURN
	iptables -t mangle -A V2RAY_MASK -d 192.168.0.0/16 -p tcp -j RETURN
	iptables -t mangle -A V2RAY_MASK -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
	iptables -t mangle -A V2RAY_MASK -j RETURN -m mark --mark 0xff
	iptables -t mangle -A V2RAY_MASK -p udp -j MARK --set-mark 1
	iptables -t mangle -A V2RAY_MASK -p tcp -j MARK --set-mark 1
	iptables -t mangle -A OUTPUT -j V2RAY_MASK


Persistant iptables

单独设置iptables，重启后会消失. 下面是持久化方法

	mkdir -p /etc/iptables && iptables-save > /etc/iptables/rules.v4

	vi /etc/systemd/system/tproxyrule.service

	[Unit]
	Description=Tproxy rule
	After=network.target
	Wants=network.target

	[Service]

	Type=oneshot
	ExecStart=/sbin/ip rule add fwmark 1 table 100 ; /sbin/ip route add local 0.0.0.0/0 dev lo table 100 ; /sbin/iptables-restore /etc/iptables/rules.v4

	[Install]
	WantedBy=multi-user.target


	systemctl enable tproxyrule

*/
package tproxy

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func HandshakeTCP(tcpConn *net.TCPConn) netLayer.Addr {
	targetTCPAddr := tcpConn.LocalAddr().(*net.TCPAddr)

	return netLayer.Addr{
		IP:   targetTCPAddr.IP,
		Port: targetTCPAddr.Port,
	}

}

var udpMsgConnMap = make(map[netLayer.HashableAddr]*MsgConn)

func HandshakeUDP(underlay *net.UDPConn) (netLayer.MsgConn, netLayer.Addr, error) {
	bs := utils.GetPacket()
	n, src, dst, err := ReadFromUDP(underlay, bs)
	if err != nil {
		return nil, netLayer.Addr{}, err
	}
	ad := netLayer.NewAddrFromUDPAddr(src)
	hash := ad.GetHashable()
	conn, found := udpMsgConnMap[hash]
	if !found {
		conn = &MsgConn{
			ourSrcAddr: src,
			readChan:   make(chan netLayer.AddrData, 5),
			closeChan:  make(chan struct{}),
		}

		udpMsgConnMap[hash] = conn

	}

	conn.readChan <- netLayer.AddrData{Data: bs[:n], Addr: netLayer.NewAddrFromUDPAddr(dst)}

	return conn, netLayer.NewAddrFromUDPAddr(dst), nil
}

//implements netLayer.MsgConn
type MsgConn struct {
	ourSrcAddr *net.UDPAddr

	readChan chan netLayer.AddrData

	closeChan chan struct{}
}

func (mc *MsgConn) Close() error {
	select {
	case <-mc.closeChan:
	default:
		close(mc.closeChan)
	}
	return nil
}

func (mc *MsgConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return nil
}

func (mc *MsgConn) Fullcone() bool {
	return true
}

func (mc *MsgConn) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	timeoutChan := time.After(netLayer.UDP_timeout)
	select {
	case <-mc.closeChan:
		return nil, netLayer.Addr{}, io.EOF
	case <-timeoutChan:
		return nil, netLayer.Addr{}, os.ErrDeadlineExceeded
	case newmsg := <-mc.readChan:
		return newmsg.Data, newmsg.Addr, nil

	}

}

// 通过透明代理 写回 客户端
func (mc *MsgConn) WriteMsgTo(p []byte, addr netLayer.Addr) error {
	back, err := DialUDP(
		"udp",
		&net.UDPAddr{
			IP:   addr.IP,
			Port: addr.Port,
		},
		mc.ourSrcAddr,
	)
	if err != nil {
		return err
	}
	_, err = back.Write(p)
	if err != nil {

		return err
	}
	back.Close()
	return nil
}
