package netLayer

import (
	"net"
	"os"
)

// 用于 listen和 dial 配置一些底层参数.
type Sockopt struct {
	TProxy bool   `toml:"tproxy"` //only linux
	Somark int    `toml:"mark"`   //only linux
	Device string `toml:"device"`

	//fastopen 不予支持, 因为自己客户端在重重网关之下，不可能让层层网关都支持tcp fast open；
	// 而自己的远程节点的话因为本来网速就很快, 也不需要fastopen，总之 因为木桶原理，慢的地方在我们层层网关, 所以fastopen 意义不大.

}

// net.TCPListener, net.UnixListener
type ListenerWithFile interface {
	net.Listener
	File() (f *os.File, err error)
}

// net.UnixConn, net.UDPConn, net.TCPConn, net.IPConn
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
