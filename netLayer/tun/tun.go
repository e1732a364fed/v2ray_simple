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
	"log"
	"net"
	"net/http"
	"net/netip"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

func ExampleTest() {
	tun, tnet, err := netstack.CreateNetTUN(
		[]netip.Addr{netip.MustParseAddr("192.168.4.29")},
		[]netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("8.8.4.4")},
		1420,
	)
	if err != nil {
		log.Panic(err)
	}
	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, ""))
	dev.IpcSet(`private_key=a8dac1d8a70a751f0f699fb14ba1cff7b79cf4fbd8f09f44c6e6a90d0369604f
public_key=25123c5dcd3328ff645e4f2a3fce0d754400d3887a0cb7c56f0267e20fbf3c5b
endpoint=163.172.161.0:12912
allowed_ip=0.0.0.0/0
persistent_keepalive_interval=25
`)
	dev.Up()
	listener, err := tnet.ListenTCP(&net.TCPAddr{Port: 80})
	if err != nil {
		log.Panicln(err)
	}
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		log.Printf("> %s - %s - %s", request.RemoteAddr, request.URL.String(), request.UserAgent())
		io.WriteString(writer, "Hello from userspace TCP!")
	})
	err = http.Serve(listener, nil)
	if err != nil {
		log.Panicln(err)
	}
}

// selfaddr是tun向外拨号时使用的ip; realAddr 是 tun接收数据时对外暴露的ip。也被称为gateway
// realAddr 是在路由表中需要配置的那个ip。
// mask是子网掩码，不是很重要.
// macos上的使用举例："", "10.1.0.10", "10.1.0.20", "255.255.255.0"
func CreateTun(name, selfaddr, realAddr, mask string, dns []string) (realname string, tunDev io.ReadWriteCloser, err error) {
	err = utils.ErrUnImplemented
	return
}

// 这个返回的closer在执行Close时可能会卡住
func ListenTun(tunDev io.ReadWriteCloser) (tcpChan <-chan netLayer.TCPRequestInfo, udpChan <-chan netLayer.UDPRequestInfo, closer io.Closer) {

	return
}
