package netLayer

import (
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
	if sockopt.Device != "" {
		bindToDevice(fd, sockopt.Device, isipv6)
	}
}

func bindToDevice(fd int, device string, is6 bool) {
	iface, err := net.InterfaceByName(device)

	if err != nil {
		if ce := utils.CanLogErr("BindToDevice failed, seems name wrong."); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if is6 {
		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, iface.Index); err != nil {
			if ce := utils.CanLogErr("BindToDevice failed, ipv6"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
	} else {

		if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, iface.Index); err != nil {
			if ce := utils.CanLogErr("BindToDevice failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
	}

	/*
		开发笔记：
		原代码与v2ray s中的代码一致，但是实测时，发现
		ipv6 绑定会报错invalid argument

		参考 https://github.com/xjasonlyu/tun2socks/pull/192 以及 wireguard-go

		原来不能两个都绑定，要依据是否为ipv6来分别处理，否则在ipv4绑定 ipv6会返回 invalid argument
	*/

}
