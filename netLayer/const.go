package netLayer

const UnknownNetwork = 0
const (
	// Transport Layer Protocols, 使用uint16 mask，所以最多支持16种
	TCP uint16 = 1 << iota
	UDP
	UNIX //unix domain socket
	Raw_socket
	KCP
	Quic //quic是一个横跨多个层的协议，这里也算一个，毕竟与kcp类似

	//一般而言，我们除了tcp和udp的协议只用于出口，不用于入口
	//不过，如果是多级代理串联的话，也会碰到需要 kcp等流量作为入口等情况。
)

//若字符串无法被解析为网络类型，则返回 UnknownNetwork
func StrToTransportProtocol(s string) uint16 {
	switch s {
	case "tcp", "tcp4", "tcp6", "TCP", "TCP4", "TCP6":
		return TCP
	case "udp", "udp4", "udp6", "UDP", "UDP4", "UDP6":
		return UDP
	case "unix", "Unix", "UNIX":
		return UNIX
	case "raw", "RAW":
		return Raw_socket
	case "kcp", "KCP":
		return KCP
	case "quic", "Quic", "QUIC":
		return Quic
	}
	return UnknownNetwork
}
