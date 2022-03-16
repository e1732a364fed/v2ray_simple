package proxy

// CMD types, for vless and vmess
const (
	_ byte = iota
	CmdTCP
	CmdUDP
	CmdMux
)
