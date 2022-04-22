package netLayer

import (
	"net"
	"os"
)

//用于 listen和 dial 配置一些底层参数.
type Sockopt struct {
	TProxy bool   `toml:"tproxy"`
	Somark int    `toml:"mark"`
	Device string `toml:"device"`
}

//net.TCPListener, net.UnixListener
type ListenerWithFile interface {
	net.Listener
	File() (f *os.File, err error)
}

//net.UnixConn, net.UDPConn, net.TCPConn, net.IPConn
type ConnWithFile interface {
	net.Conn
	File() (f *os.File, err error)
}

func SetSockOptForListener(tcplistener ListenerWithFile, sockopt *Sockopt, isudp bool, isipv6 bool) {
	fileDescriptorSource, err := tcplistener.File()
	if err != nil {
		return
	}
	defer fileDescriptorSource.Close()
	SetSockOpt(int(fileDescriptorSource.Fd()), sockopt, isudp, isipv6)
}

//SetSockOpt 是平台相关的.
