/*Package socks5http provides listening both socks5 and http at one port.

This package imports proxy/socks5 and proxy/http package.

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
	"go.uber.org/zap"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

const Name = "socks5http"

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{}

func (ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return newServer(), nil
}

func (ServerCreator) NewServer(dc *proxy.ListenConf) (proxy.Server, error) {
	return newServer(), nil
}

func newServer() *Server {
	return &Server{
		hs: http.Server{
			//OnlyConnect: true,	//之前本以为connect就可以搞定一切，后来实测发现 wget 确实在 非https时 会用 纯http请求的方式 请求代理。
		},
	}
}

type Server struct {
	proxy.Base

	hs http.Server
	ss socks5.Server
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

		if ce := utils.CanLogDebug("socks5http: http failed, will try socks5"); ce != nil {
			ce.Write(zap.Int("buflen", buf.Len()))
		}

		newConn := &netLayer.ReadWrapper{
			Conn:              underlay,
			OptionalReader:    io.MultiReader(buf, underlay),
			RemainFirstBufLen: buf.Len(),
		}

		return s.ss.Handshake(newConn)
	}

	return
}
