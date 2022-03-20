// Pakcage ws implements websocket handshake
//
//websocket rfc: https://datatracker.ietf.org/doc/html/rfc6455/
package ws

import (
	"context"
	"log"
	"net"
	"net/url"

	"github.com/gobwas/ws"
	"github.com/hahahrfool/v2ray_simple/utils"
)

/*
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
*/

type Client struct {
	//path     string
	//hostAddr string //host+port, port 可省略

	//wholePath string // www.google.com/chat1

	requestURL *url.URL
}

func NewClient(hostAddr string, path string) (*Client, error) {
	u, err := url.Parse("http://" + hostAddr + "/" + path)
	if err != nil {
		return nil, err
	}
	return &Client{
		requestURL: u,
	}, nil
}

func (c *Client) Handshake(underlay net.Conn) (conn net.Conn, err error) {
	d := ws.Dialer{
		//ReadBufferSize:  readBufSize,
		//WriteBufferSize: writeBufSize,
		NetDial: func(ctx context.Context, net, addr string) (net.Conn, error) {
			return underlay, nil
		},
	}

	br, hs, err := d.Upgrade(underlay, c.requestURL)
	log.Println("ws:", conn, br, hs, err)
	if err != nil {
		return nil, err
	}
	//之所以被返回了br，是因为服务器可能紧接着想我们迅猛地发送数据;
	// 比如如果是vless v0，服务器就会返回一个响应头
	// 但是，作为一个代理服务器，某种代理协议 有可能没有任何响应，而静静等待新数据传来

	//另外，根据 gobwas/ws的代码，在服务器没有返回任何数据时，br为nil
	if br == nil {
		return &Conn{
			Conn:     underlay,
			notFirst: true,
		}, nil
	}

	additionalDataNum := br.Buffered()
	bs, _ := br.Peek(additionalDataNum)
	return &Conn{
		firstData: bs,
		Conn:      underlay,
	}, nil
}

type Conn struct {
	net.Conn
	firstData []byte
	notFirst  bool
}

func (c *Conn) Read(p []byte) (int, error) {
	if c.notFirst {
		return c.read(p)
	}
	bs := c.firstData
	c.firstData = nil
	c.notFirst = true

	n := copy(p, bs)
	utils.PutBytes(bs)
	return n, nil
}

//读取binary
func (c *Conn) read(p []byte) (int, error) {

	return 0, nil
}

//Write binary
func (c *Conn) Write(p []byte) (int, error) {

	return 0, nil
}
