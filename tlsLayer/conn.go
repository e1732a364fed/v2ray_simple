package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"
)

//参考 crypt/tls 的 conn.go， 注意，如果上游代码的底层结构发生了改变，则这里也要跟着修改，保持头部结构一致
type faketlsconn struct {
	// constant
	conn     net.Conn
	isClient bool
}

// 本包会用到这个Conn，比如server和client的 Handshake，

//唯一特性就是它可以返回tls连接的底层tcp连接，见 GetRaw
// 开启了回落功能的话，这里还会用到 http.sniff
type Conn struct {
	*tls.Conn
}

func (c *Conn) GetRaw(tls_lazy_encrypt bool) *net.TCPConn {
	rc := (*faketlsconn)(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn))))
	if rc != nil {
		if rc.conn != nil {
			//log.Println("成功获取到 *net.TCPConn！", rc.conn.(*net.TCPConn)) //经测试，是毫无问题的，完全能提取出来并正常使用
			//在 tls_lazy_encrypt 时，我们使用 TeeConn

			if tls_lazy_encrypt {
				tc := rc.conn.(*TeeConn)
				return tc.OldConn.(*net.TCPConn)
			} else {
				return rc.conn.(*net.TCPConn)
			}

		}
	}
	return nil
}

// 直接获取TeeConn，仅用于已经确定肯定能获取到的情况
func (c *Conn) GetTeeConn() *TeeConn {
	rc := (*faketlsconn)(unsafe.Pointer(uintptr(unsafe.Pointer(c.Conn))))

	return rc.conn.(*TeeConn)

}
