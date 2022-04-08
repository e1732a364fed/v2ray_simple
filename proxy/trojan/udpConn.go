package trojan

import (
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

type UDPConn struct {
	net.Conn
}

func (u UDPConn) Fullcone() bool {
	return true
}
func (u UDPConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return u.Close()
}
func (u UDPConn) ReadFrom() ([]byte, netLayer.Addr, error) {

	return nil, netLayer.Addr{}, nil
}

func (u UDPConn) WriteTo([]byte, netLayer.Addr) error {

	return nil
}
