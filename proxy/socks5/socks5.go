/*
Package socks5 provies socks5 proxy for proxy.Client and proxy.Server.

Supports USER/PASSWORD authentication.

# Reference

English: https://www.ietf.org/rfc/rfc1928.txt

中文： https://aber.sh/articles/Socks5/

参考 https://studygolang.com/articles/31404

USER/PASSWORD authentication rfc:

https://datatracker.ietf.org/doc/html/rfc1929

注意，socks5可能同时使用tcp和udp，但是一定会使用到tcp，socks5的network只能设置为tcp或者dual

# Off Topic

纵观各种代理协议，vless/vmess/trojan/shadowsocks协议 都借鉴了socks5，有不少类似的地方。
所以 制作代理, 有必要先学习socks5标准。

关于socks4, 它太简单了, 既不支持udp, 也不支持ipv6, 也没有验证功能, 所以本作不予支持
*/
package socks5

const Name = "socks5"

// socks5 version number.
const Version5 = 0x05

// SOCKS auth type
const (
	AuthNone     = 0x00
	AuthPassword = 0x02

	AuthNoACCEPTABLE = 0xff
)

// SOCKS request commands as defined in RFC 1928 section 4
const (
	CmdConnect      = 0x01
	CmdBind         = 0x02
	CmdUDPAssociate = 0x03
)

// SOCKS address types as defined in RFC 1928 section 4
//
//	Note: vmess/vless用的是123，而这里用的是134，所以是不一样的。
const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
)
