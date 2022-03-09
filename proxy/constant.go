package proxy

// CMD types, for vless and vmess
const (
	_ byte = iota
	CmdTCP
	CmdUDP
	CmdMux
)

// Atyp, for vless and vmess; 注意与 trojan和socks5的区别，trojan和socks5的相同含义的值是1，3，4
const (
	AtypIP4    byte = 1
	AtypDomain byte = 2
	AtypIP6    byte = 3
)
