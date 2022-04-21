package netLayer

import (
	"golang.org/x/sys/unix"
	"syscall"
)

func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
	if sockopt == nil {
		return
	}

	if sockopt.Somark != 0 {
		setSomark(int(fd), sockopt.Somark)
	}

	if sockopt.TProxy {
		setTproxy(int(fd))

		if isudp {
			setTproxy_udp(int(fd), isipv6)
		}
	}

}

func setTproxy(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
}

func setTproxy_udp(fd int, isipv6 bool) error {
	err1 := syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
	if err1 != nil {
		return err1
	}
	if isipv6 {
		return syscall.SetsockoptInt(int(fd), syscall.SOL_IPV6, unix.IPV6_RECVORIGDSTADDR, 1)
	}
	return nil
}

func setSomark(fd int, somark int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, somark)
}
