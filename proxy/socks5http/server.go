/*Package socks5http provides listening both socks5 and http at one port.

This package imports proxy/socks5 and proxy/http package.

Naming

socks5http 与 clash的 "mixed" 等价。之所以不用 "mixed"这个名称，是因为这容易在本作中引起歧义。

clash是一个客户端，它没有服务端，所以它的监听只是用于内网监听，所以监听协议  只有http和socks5 两种，所以 它 叫 "mixed" 是没有歧义的；

而本作与v2ray一样，是支持多种服务端协议的，如果也叫 mixed 的话，会让人误以为，这是一个 "万能协议", 啥都能监听， 而这显然是误区。 命名为 socks5http, 则清晰地指出了 该协议的功能。

Password

为了避免混淆，本包不支持密码验证。你要是有这么高的密码要求 那你不妨用单独的协议，而不要用混合版。

实际上本包就是先经过http，然后如果不是http代理请求，就会回落到socks5.

所以你可以通过 设计回落的方式来达到 有密码 的 混合端口 的需求。


*/
package socks5http

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/proxy/http"
	"github.com/e1732a364fed/v2ray_simple/proxy/socks5"
	"github.com/e1732a364fed/v2ray_simple/utils"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

const Name = "socks5http"

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{}

func (ServerCreator) URLToListenConf(u *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {

	if lc == nil {
		lc = &proxy.ListenConf{}
	}

	return lc, nil

}

func (ServerCreator) NewServer(dc *proxy.ListenConf) (proxy.Server, error) {
	return newServer(), nil
}

func newServer() *Server {
	return &Server{
		hs: http.NewServer(),
		ss: socks5.NewServer(),
	}
}

type Server struct {
	proxy.Base

	hs *http.Server
	ss *socks5.Server
}

func (*Server) Name() string {
	return Name
}

func (s *Server) Handshake(underlay net.Conn) (newconn net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, err error) {
	newconn, _, targetAddr, err = s.hs.Handshake(underlay)
	if err == nil {
		return
	}

	if be, ok := err.(utils.ErrBuffer); ok {
		buf := be.Buf

		//if ce := utils.CanLogDebug("socks5http: http failed, will try socks5"); ce != nil {
		//	ce.Write(zap.Int("buflen", buf.Len()))
		//}

		newConn := &netLayer.ReadWrapper{
			Conn:              underlay,
			OptionalReader:    io.MultiReader(buf, underlay),
			RemainFirstBufLen: buf.Len(),
		}

		return s.ss.Handshake(newConn)
	}

	return
}
