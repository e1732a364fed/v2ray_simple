/*
Package ws implements websocket for advLayer.

# Reference

websocket rfc: https://datatracker.ietf.org/doc/html/rfc6455/

Below is a real websocket handshake progress:

Request

	GET /chat HTTP/1.1
	    Host: server.example.com
	    Upgrade: websocket
	    Connection: Upgrade
	    Sec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==
	    Sec-WebSocket-Protocol: chat, superchat
	    Sec-WebSocket-Version: 13
	    Origin: http://example.com

Response

	HTTP/1.1 101 Switching Protocols
	    Upgrade: websocket
	    Connection: Upgrade
	    Sec-WebSocket-Accept: HSmrc0sMlYUkAGmm5OPpG2HaGWk=
	    Sec-WebSocket-Protocol: chat

websocket packages comparison:
https://yalantis.com/blog/how-to-build-websockets-in-go/

中文翻译：
https://tonybai.com/2019/09/28/how-to-build-websockets-in-go/

All in all gobwas/ws is the best package. We use gobwas/ws.

gobwas包只支持http1.1, 所以如果使用nginx前置，确保 proxy_http_version 1.1;
*/
package ws

import (
	"github.com/e1732a364fed/v2ray_simple/advLayer"
)

// 2048 /3 = 682.6666...  (682 又 三分之二),
// 683 * 4 = 2732, 你若不信，运行 we_test.go中的 TestBase64Len
const MaxEarlyDataLen_Base64 = 2732
const MaxEarlyDataLen = 2048

func init() {
	advLayer.ProtocolsMap["ws"] = Creator{}
}

type Creator struct{}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {
	hn := conf.Host
	if conf.Addr.Network == "unix" {
		hn = ""
	}
	return NewClient(hn, conf.Path, conf.Headers, conf.IsEarly)
}

func (Creator) NewServerFromConf(conf *advLayer.Conf) (advLayer.Server, error) {
	return NewServer(conf.Path, conf.Headers, conf.IsEarly), nil
}
func (Creator) GetDefaultAlpn() (alpn string, mustUse bool) {
	return
}
func (Creator) PackageID() string {
	return "ws"
}

func (Creator) ProtocolName() string {
	return "ws"
}

func (Creator) CanHandleHeaders() bool {
	return true
}

func (Creator) IsMux() bool {
	return false
}

func (Creator) IsSuper() bool {
	return false
}
