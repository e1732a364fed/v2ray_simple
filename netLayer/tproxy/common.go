/*
Package tproxy listens tproxy and setup corresponding iptables for linux.

vs的透明代理只能用于linux 和 macos

原理上，透明代理与tun/tap不同，透明代理直接工作在传输层第四层tcp/udp上，无需解析ip包

下面的文档首先探讨了linux的tproxy

# About TProxy 关于透明代理

透明代理原理
https://www.kernel.org/doc/html/latest/networking/tproxy.html

https://powerdns.org/tproxydoc/tproxy.md.html

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

同时，trojan-go还使用了.
https://github.com/cybozu-go/transocks/blob/master/original_dst_linux.go

不过实测我们不需要用这个代码来获取原始地址，因为地址我们直接就从 localAddr就能获取。也许是trojan-go的作者不懂tproxy的原理吧！

# Iptables

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

# Persistent iptables

单独设置iptables，重启后会消失. 下面是 有systemd 的系统的 持久化方法

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

# OffTopic

透明代理与Redir的参考博客：

http://ivo-wang.github.io/2018/02/24/ss-redir/

# 关于mac上的实现

我参考了如下内容

https://penglei.github.io/post/transparent_proxy_on_macosx/
https://docs.mitmproxy.org/stable/howto-transparent/#macos
https://github.com/Dreamacro/clash/issues/745
https://github.com/shadowsocks/go-shadowsocks2

mac上的透明代理使用了pf命令，通过一条命令来将全局流量导向特定端口；
不过只支持ipv4和tcp

sudo sysctl -w net.inet.ip.forwarding=1

创建一个文件，pf.conf

rdr pass on en0 inet proto tcp to any port {80, 443} -> 127.0.0.1 port 8080

sudo pfctl -f pf.conf
sudo pfctl -e

查看效果
sudo pfctl -s nat

如果想停止使用透明代理访问，禁用pf(sudo pfctl -d)或者清空pf规则(sudo pfctl -F all)即可。

我试了，mac12.0 ～ mac13 的 pfctl有问题，设路由不好使。参考
https://github.com/mitmproxy/mitmproxy/issues/4835
*/
package tproxy

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

// implements netLayer.MsgConn
type MsgConn struct {
	netLayer.EasyDeadline

	parentMachine *Machine

	hash netLayer.HashableAddr

	ourSrcAddr *net.UDPAddr

	readChan chan netLayer.AddrData

	closeChan chan struct{}

	fullcone bool
}

// 一个tproxy状态机 具有 监听端口、tcplistener、udpConn 这三个要素。
// 用于关闭 以及 储存所监听的 端口。
type Machine struct {
	netLayer.Addr
	net.Listener //tcpListener
	*net.UDPConn

	udpMsgConnMap map[netLayer.HashableAddr]*MsgConn
	sync.RWMutex  //避免存储 与 移除  产生多线程冲突

	iptablePort int
	closed      bool
}

func NewMachine() *Machine {
	return &Machine{
		udpMsgConnMap: make(map[netLayer.HashableAddr]*MsgConn),
	}
}

func (m *Machine) Closed() bool {
	return m.closed
}

func (m *Machine) Init() {
	m.udpMsgConnMap = make(map[netLayer.HashableAddr]*MsgConn)
}

func (m *Machine) SetIPTable(port int) {
	if port > 0 {
		SetRouteByPort(port)
		m.iptablePort = port

	}
}

func (m *Machine) Stop() {
	m.closed = true
	if m.Listener != nil {
		log.Println("closing tproxy listener")
		//后来发现，不知为何，这个 Close调用会卡住

		ch := make(chan struct{})
		go func() {
			m.Listener.Close()
			close(ch)
		}()
		tCh := time.After(time.Second)
		select {
		case <-tCh:
			log.Println("tproxy close listener timeout")
		case <-ch:
			break
		}

	}
	if m.UDPConn != nil {
		log.Println("tproxy closing udp conn")

		m.UDPConn.Close()

	}
	if m.iptablePort > 0 {
		CleanupRoutes()
	}
}
