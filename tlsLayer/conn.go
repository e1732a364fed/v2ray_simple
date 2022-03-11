package tlsLayer

import (
	"crypto/tls"
	"log"
	"net"
	"unsafe"
)

type Conn struct {
	*tls.Conn
}

type faketlsconn struct {
	// constant
	conn     net.Conn
	isClient bool
}

func (c *Conn) GetRaw() *net.TCPConn {
	rc := (*faketlsconn)(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn))))
	if rc != nil {
		if rc.conn != nil {
			log.Println("成功获取到 *net.TCPConn！", rc.conn.(*net.TCPConn))
			return rc.conn.(*net.TCPConn)
		}
	}
	return nil
}
