package trojan

import "net"

type UDPConn struct {
	net.Conn
}
