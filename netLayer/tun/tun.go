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

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

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
