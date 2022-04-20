package netLayer

import (
	"syscall"
)

func SetTproxyFor(tcplistener ListenerWithFile) error {
	fileDescriptorSource, err := tcplistener.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return syscall.SetsockoptInt(int(fileDescriptorSource.Fd()), syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
}

func SetSomarkForListener(tcplistener ListenerWithFile, somark int) error {
	fileDescriptorSource, err := tcplistener.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return syscall.SetsockoptInt(int(fileDescriptorSource.Fd()), syscall.SOL_SOCKET, syscall.SO_MARK, somark)
}

func SetSomarkForConn(c ConnWithFile) error {
	fileDescriptorSource, err := c.File()
	if err != nil {
		return err
	}
	defer fileDescriptorSource.Close()

	return syscall.SetsockoptInt(int(fileDescriptorSource.Fd()), syscall.SOL_SOCKET, syscall.SO_MARK, int(config.Mark))
}
