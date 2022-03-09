package socks5

//总体而言，vless和vmess协议借鉴了socks5，所以有类似的地方。trojan协议也是一样。

const Name = "socks5"

// https://www.ietf.org/rfc/rfc1928.txt

// Version is socks5 version number.
const Version5 = 0x05

// SOCKS auth type
const (
	AuthNone     = 0x00
	AuthPassword = 0x02
)

// SOCKS request commands as defined in RFC 1928 section 4
const (
	CmdConnect      = 0x01
	CmdBind         = 0x02
	CmdUDPAssociate = 0x03
)

// SOCKS address types as defined in RFC 1928 section 4
//	Note: vmess/vless用的是123，而这里用的是134，所以是不一样的。
const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
)
