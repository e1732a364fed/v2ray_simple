/*
Packages tun provides utilities for tun.
tun 工作在第三层 IP层上。

我们监听tun，从中提取出 tcp/udp 流。

我们使用 github.com/eycorsican/go-tun2socks 包

# Problem 问题

这个包在windows上会使用tap。

目前测试在windows上效果非常不好，响应很慢，似乎和udp或者dns有一定关联.
它总在访问组播地址 239.255.255.250

eycorsican/go-tun2socks 包问题不小，不仅有平台间不一致的问题，而且tun关闭后无法再重新开启
*/
package tun

import (
	"io"
	"net"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/tun"
	"github.com/songgao/water"
	"go.uber.org/zap"
)

// implements netLayer.MsgConn
type coreUDPConnAdapter struct {
	core.UDPConn
	netLayer.EasyDeadline

	readChan           chan netLayer.UDPAddrData
	firstRemoteUDPAddr *net.UDPAddr
}

func newUdpAdapter() *coreUDPConnAdapter {
	c := new(coreUDPConnAdapter)
	c.InitEasyDeadline()
	c.readChan = make(chan netLayer.UDPAddrData, 1)
	return c
}

func (h *coreUDPConnAdapter) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	ud := <-h.readChan

	if h.firstRemoteUDPAddr == nil {
		h.firstRemoteUDPAddr = &ud.Addr
	}

	return ud.Data, netLayer.NewAddrFromUDPAddr(&ud.Addr), nil
}
func (h *coreUDPConnAdapter) WriteMsgTo(data []byte, ad netLayer.Addr) error {
	ua := ad.ToUDPAddr()
	if ua == nil {
		ua = h.firstRemoteUDPAddr
	}
	_, err := h.UDPConn.WriteFrom(data, ua)
	return err
}

func (h *coreUDPConnAdapter) CloseConnWithRaddr(raddr netLayer.Addr) error {

	return nil
}
func (h *coreUDPConnAdapter) Fullcone() bool {
	return true
}

type handler struct {
	tcpChan chan netLayer.TCPRequestInfo
	udpChan chan netLayer.UDPRequestInfo

	udpmap map[netLayer.HashableAddr]*coreUDPConnAdapter
	sync.RWMutex
}

func newHandler() *handler {
	return &handler{
		tcpChan: make(chan netLayer.TCPRequestInfo),
		udpChan: make(chan netLayer.UDPRequestInfo),
		udpmap:  make(map[netLayer.HashableAddr]*coreUDPConnAdapter),
	}
}

func (h *handler) Handle(conn net.Conn, target *net.TCPAddr) error {
	tad := netLayer.NewAddrFromTCPAddr(target)
	//实测 这里 target 就是 conn.RemoteAddr()

	h.tcpChan <- netLayer.TCPRequestInfo{Target: tad, Conn: conn}
	return nil
}

func (h *handler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	return nil
}

func (h *handler) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	//log.Println("ReceiveTo called")

	//这个conn是 tun的conn，我们只调用它的 WriteFrom 方法 把从外部获得的数据写入 tunDev

	//也就是说，netLayer.MsgConn.ReadMsgFrom获得的数据要用 core.UDPConn.WriteFrom 写入

	//tun 会调用我们的 ReceiveTo 方法 给我们新的 从tun读到的消息

	uad := netLayer.NewAddrFromUDPAddr(addr)

	ha := uad.GetHashable()
	h.RLock()

	if adapter, ok := h.udpmap[ha]; ok {
		h.RUnlock()
		adapter.readChan <- netLayer.UDPAddrData{Data: data, Addr: *addr}

	} else {
		h.RUnlock()
		adapter := newUdpAdapter()
		adapter.UDPConn = conn

		h.Lock()
		h.udpmap[ha] = adapter
		h.Unlock()

		adapter.readChan <- netLayer.UDPAddrData{Data: data, Addr: *addr}

		h.udpChan <- netLayer.UDPRequestInfo{Target: uad, MsgConn: adapter}
	}

	return nil
}

// selfaddr是tun向外拨号时使用的ip; realAddr 是 tun接收数据时对外暴露的ip。也被称为gateway
// realAddr 是在路由表中需要配置的那个ip。
// mask是子网掩码，不是很重要.
// macos上的使用举例："", "10.1.0.10", "10.1.0.20", "255.255.255.0"
func CreateTun(name, selfaddr, realAddr, mask string, dns []string) (realname string, tunDev io.ReadWriteCloser, err error) {
	//tun.OpenTunDevice 是一个非常平台相关的函数，而且看起来不是太完美

	//在windows上调用时，dns不能为空, 否则闪退; 而在macos上 dns 会被无视; 在windows上调用返回的是一个tap设备，而不是tun
	//在linux上调用时 addr, gw, mask, dnsServers 都会被无视

	//macos 上无法指定tun名称
	if len(dns) == 0 {
		dns = []string{"114.114.114.114"}
	}
	tunDev, err = tun.OpenTunDevice(name, selfaddr, realAddr, mask, dns, false)
	if err == nil {
		wi, ok := tunDev.(*water.Interface)
		if ok {
			realname = wi.Name()
		} else {
			realname = name
		}
		if ce := utils.CanLogInfo("created new tun device"); ce != nil {
			ce.Write(
				zap.String("name", realname),
				zap.String("gateway", realAddr),
				zap.String("selfip", selfaddr),
				zap.String("mask", mask),
			)
		}
	}
	/*
		如果你是 tun listen, direct dial,
		要配置好路由表；如果默认情况的话，会造成无限本地回环，因为我们代理发出的流量又被导向代理本身


	*/

	return
}

// 这个返回的closer在执行Close时可能会卡住
func ListenTun(tunDev io.ReadWriteCloser) (tcpChan <-chan netLayer.TCPRequestInfo, udpChan <-chan netLayer.UDPRequestInfo, closer io.Closer) {
	lwip := core.NewLWIPStack()
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})
	nh := newHandler()
	core.RegisterTCPConnHandler(nh)
	core.RegisterUDPConnHandler(nh)
	go func() {
		_, err := io.CopyBuffer(lwip, tunDev, make([]byte, utils.MTU))
		if err != nil {
			if ce := utils.CanLogWarn("tun copying from tunDev to lwip failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
	}()
	tcpChan = nh.tcpChan
	udpChan = nh.udpChan
	closer = lwip
	return
}
