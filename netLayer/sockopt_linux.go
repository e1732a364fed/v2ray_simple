package netLayer

import (
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
	if sockopt == nil {
		return
	}

	if sockopt.Somark != 0 {
		setSomark(fd, sockopt.Somark)
	}

	if sockopt.TProxy {
		setTproxy(fd)

		if isudp {
			setTproxy_udp(fd, isipv6)
		}
	}

	if sockopt.Device != "" {
		bindToDevice(fd, sockopt.Device)
	}

	if sockopt.BBR {
		if err := syscall.SetsockoptString(fd, syscall.SOL_TCP, syscall.TCP_CONGESTION, "bbr"); err != nil {
			if ce := utils.CanLogErr("bbr failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}
	}

}

func bindToDevice(fd int, device string) {
	if err := unix.BindToDevice(fd, device); err != nil {
		if ce := utils.CanLogErr("BindToDevice failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
	}
}

func setTproxy(fd int) {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1); err != nil {
		if ce := utils.CanLogErr("setTproxy failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
	}
}

func setTproxy_udp(fd int, isipv6 bool) {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1); err != nil {
		if ce := utils.CanLogErr("set IP_RECVORIGDSTADDR failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if isipv6 {
		if err := syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_RECVORIGDSTADDR, 1); err != nil {
			if ce := utils.CanLogErr("set IPV6_RECVORIGDSTADDR failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
		}
	}
}

func setSomark(fd int, somark int) {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, somark); err != nil {
		if ce := utils.CanLogErr("setSomark failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
	}
}
