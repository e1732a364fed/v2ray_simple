package tproxy

import (
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

type Machine struct {
	netLayer.Addr
	net.Listener //tcpListener
	*net.UDPConn
}

func (m *Machine) Stop() {
	if m.Listener != nil {
		m.Listener.Close()

	}
	if m.UDPConn != nil {
		m.UDPConn.Close()

	}
}
