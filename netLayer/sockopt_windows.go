package netLayer

import (
	"encoding/binary"
	"net"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// SetSockOpt 是平台相关的.
func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
	if sockopt.Device != "" {
		bindToDevice(fd, sockopt.Device, isipv6)
	}
}

//相关讨论参考 https://github.com/xjasonlyu/tun2socks/pull/192

func bindToDevice(fd int, device string, is6 bool) {
	iface, err := net.InterfaceByName(device)
	if err != nil {
		if ce := utils.CanLogErr("BindToDevice failed, seems name wrong."); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	const (
		IP_UNICAST_IF   = 31
		IPV6_UNICAST_IF = 31
	)

	if is6 {
		if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IPV6, IPV6_UNICAST_IF, iface.Index); err != nil {
			if ce := utils.CanLogErr("BindToDevice failed, ipv6"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
	} else {

		//https://github.com/xjasonlyu/tun2socks/pull/192/files

		// For IPv4, this parameter must be an interface index in network byte order.
		// Ref: https://learn.microsoft.com/en-us/windows/win32/winsock/ipproto-ip-socket-options

		var bytes [4]byte
		binary.BigEndian.PutUint32(bytes[:], uint32(iface.Index))
		index := *(*uint32)(unsafe.Pointer(&bytes[0]))

		if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, IP_UNICAST_IF, int(index)); err != nil {
			if ce := utils.CanLogErr("BindToDevice failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
	}

}
