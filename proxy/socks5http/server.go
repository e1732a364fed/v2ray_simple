/*Package socks5http provides listening both socks5 and http at one port.

This package imports proxy/socks5 and proxy/http package.
*/
package socks5http

import (
	"io"
	"log"
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

func (ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return newServer(), nil
}

func (ServerCreator) NewServer(dc *proxy.ListenConf) (proxy.Server, error) {
	return newServer(), nil
}

func newServer() *Server {
	return &Server{
		hs: http.Server{
			MustFallback: true,
			OnlyConnect:  true,
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

		log.Println("socks5http: http failed, will try socks5", buf.Len())

		newConn := &netLayer.ReadWrapper{
			Conn:              underlay,
			OptionalReader:    io.MultiReader(buf, underlay),
			RemainFirstBufLen: buf.Len(),
		}

		return s.ss.Handshake(newConn)
	}

	return
}
