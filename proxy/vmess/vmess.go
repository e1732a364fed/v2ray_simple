/*
Package vmess implements vmess for proxy.Client and proxy.Server.

from github.com/Dreamacro/clash/tree/master/transport/vmess/

本作不支持alterid!=0 的情况. 即 仅支持 使用 aead 方式 进行认证. 即不支持 "MD5 认证信息"

标准:  https://www.v2fly.org/developer/protocols/vmess.html

aead:

https://github.com/v2fly/v2fly-github-io/issues/20

# Implementation Details

vmess 协议是一个很老旧的协议，有很多向前兼容的代码，很多地方都已经废弃了. 我们这里只支持最新的aead.

我们所实现的vmess 服务端 力求简单、最新，不求兼容所有老旧客户端。

# Share URL

v2fly只有一个草案
https://github.com/v2fly/v2fly-github-io/issues/26

似乎v2fly社区对于这个URL标准的制定并不注重，而且看起来这个草案也不太美观

而xray社区的则美观得多，见 https://github.com/XTLS/Xray-core/discussions/716
*/
package vmess

const Name = "vmess"

// Request Options
const (
	OptBasicFormat byte = 0 // 基本格式
	OptChunkStream byte = 1 // 标准格式,实际的请求数据被分割为若干个小块

	OptChunkMasking byte = 4

	OptGlobalPadding byte = 0x08

	//OptAuthenticatedLength byte = 0x10
)

// Security types
const (
	SecurityAES256GCM        byte = 3
	SecurityChacha20Poly1305 byte = 4
	SecurityNone             byte = 5
)

// v2ray CMD types
const (
	CmdTCP                    byte = 1
	CmdUDP                    byte = 2
	cmd_muxcool_unimplemented byte = 3 //mux.cool的command的定义 在 v2ray源代码的 common/protocol/headers.go 的 RequestCommandMux。

	CMDMux_VS byte = 4 //新定义的值，用于使用我们vs的mux方式
)

// var getkeyBs = []byte("c48619fe-8f02-49e0-b9e9-edf763e17e21")
