package netLayer

import (
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
	if sockopt.Device != "" {
		bindToDevice(fd, sockopt.Device)
	}
}

func bindToDevice(fd int, device string) {
	iface, err := net.InterfaceByName(device)

	if err != nil {
		if ce := utils.CanLogErr("BindToDevice failed, seems name wrong."); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index); err != nil {
		if ce := utils.CanLogErr("BindToDevice failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	/*
		开发笔记：
		这些代码与v2ray或者 tun2socks中的代码一致，但是实测时，发现
		ipv6 绑定会报错invalid argument
	*/

	//invalid argument
	// if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, iface.Index); err != nil {
	// 	if ce := utils.CanLogErr("BindToDevice failed, ipv6"); ce != nil {
	// 		ce.Write(zap.Error(err))
	// 	}
	// 	return
	// }
}
