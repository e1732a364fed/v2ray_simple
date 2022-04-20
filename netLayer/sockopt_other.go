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
