package netLayer

const (
	// transport Layer, 使用uint16 mask，所以最多支持16种

	TCP uint16 = 1 << iota
	UDP
	UNIX //unix domain socket
	Raw_socket
	KCP
	Quic //quic是一个横跨多个层的协议，这里也算一个，毕竟与kcp类似

	//一般而言，我们除了tcp和udp的协议只用于出口，不用于入口
	//不过，如果是多级代理串联的话，也会碰到需要 kcp等流量作为入口等情况。
)

func StrToTransportProtocol(s string) uint16 {
	switch s {
	case "tcp":
		return TCP
	case "udp":
		return UDP
	case "unix":
		return UNIX
	case "raw":
		return Raw_socket
	case "kcp":
		return KCP
	case "quic":
		return Quic

	}
	return 0
}
