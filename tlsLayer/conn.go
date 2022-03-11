package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"
)

// 本包会用到这个Conn，唯一特性就是它可以返回tls连接的底层tcp连接，见 GetRaw
type Conn struct {
	*tls.Conn
}

//参考 crypt/tls 的 conn.go， 注意，如果上游代码的底层结构发生了改变，则这里也要跟着修改，保持头部结构一致
type faketlsconn struct {
	// constant
	conn     net.Conn
	isClient bool
}

func (c *Conn) GetRaw() *net.TCPConn {
	rc := (*faketlsconn)(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn))))
	if rc != nil {
		if rc.conn != nil {
			//log.Println("成功获取到 *net.TCPConn！", rc.conn.(*net.TCPConn)) //经测试，是毫无问题的，完全能提取出来并正常使用
			return rc.conn.(*net.TCPConn)
		}
	}
	return nil
}
