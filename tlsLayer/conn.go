package tlsLayer

import (
	"crypto/tls"
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
			//log.Println("成功获取到 *net.TCPConn！", rc.conn.(*net.TCPConn)) //经测试，是毫无问题的
			return rc.conn.(*net.TCPConn)
		}
	}
	return nil
}
