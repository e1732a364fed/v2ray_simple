/*Package ws implements websocket handshake.

Reference

websocket rfc: https://datatracker.ietf.org/doc/html/rfc6455/

下面把一个握手放在这里作为参考

请求
GET /chat HTTP/1.1
    Host: server.example.com
    Upgrade: websocket
    Connection: Upgrade
    Sec-WebSocket-Key: x3JJHMbDL1EzLkh9GBhXDw==
    Sec-WebSocket-Protocol: chat, superchat
    Sec-WebSocket-Version: 13
    Origin: http://example.com

响应
HTTP/1.1 101 Switching Protocols
    Upgrade: websocket
    Connection: Upgrade
    Sec-WebSocket-Accept: HSmrc0sMlYUkAGmm5OPpG2HaGWk=
    Sec-WebSocket-Protocol: chat

总之，一个websocket的请求头直接就是一个 合法的http请求头，所以也没必要额外包一个http连接，
直接使用tcp/tls 连接即可。

websocket 库比较 https://yalantis.com/blog/how-to-build-websockets-in-go/

中文翻译：
https://tonybai.com/2019/09/28/how-to-build-websockets-in-go/

总之 gobwas/ws 是最好的库. 本包使用 gobwas/ws
*/
package ws

import "github.com/e1732a364fed/v2ray_simple/advLayer"

func init() {
	advLayer.ProtocolsMap["ws"] = Creator{}
}

type Creator struct{}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {
	return NewClient(conf.Host, conf.Path, conf.Headers, conf.IsEarly)
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
