package netLayer

import (
	"syscall"
)

func SetTproxy(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
}

func SetTproxy_udp(fd int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_IP, syscall.IP_RECVORIGDSTADDR, 1)
}

func SetSomark(fd int, somark int) error {
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, somark)
}

func SetTproxyFor(tcplistener ListenerWithFile) error {
	fileDescriptorSource, err := tcplistener.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return SetTproxy(int(fileDescriptorSource.Fd()))
}

func SetSomarkForListener(tcplistener ListenerWithFile, somark int) error {
	fileDescriptorSource, err := tcplistener.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return SetSomark(int(fileDescriptorSource.Fd()), somark)
}

func SetSomarkForConn(c ConnWithFile, somark int) error {
	fileDescriptorSource, err := c.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return SetSomark(int(fileDescriptorSource.Fd()), somark)
}
