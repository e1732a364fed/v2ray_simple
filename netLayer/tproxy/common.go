package tproxy

import (
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

//一个tproxy状态机 具有 监听端口、tcplistener、udpConn 这三个要素。
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
