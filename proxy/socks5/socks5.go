/*Package socks5 provies socks5 proxy for proxy.Client and proxy.Server.

Supports USER/PASSWORD authentication.

Reference

English: https://www.ietf.org/rfc/rfc1928.txt

中文： https://aber.sh/articles/Socks5/

参考 https://studygolang.com/articles/31404

USER/PASSWORD authentication rfc:

https://datatracker.ietf.org/doc/html/rfc1929

Off Topic

总体而言，vless/vmess/trojan协议借鉴了socks5，有不少类似的地方。
所以制作代理, 有必要学习socks5标准。
*/
package socks5

const Name = "socks5"

//socks5 version number.
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
//	Note: vmess/vless用的是123，而这里用的是134，所以是不一样的。
const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
)
