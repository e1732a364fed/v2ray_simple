//go:build !linux
// +build !linux

package netLayer

func SetTproxyFor(tcplistener ListenerWithFile) error {
	return nil
}

func SetSomarkForListener(tcplistener ListenerWithFile) error {
	return nil
}

func SetSomarkForConn(c ConnWithFile) error {
	return nil
}

func SetTproxy(fd int) error {
	return nil
}

func SetSomark(fd int, somark int) error {
	return nil
}
func SetTproxy_udp(fd int) error {
	return nil
}
