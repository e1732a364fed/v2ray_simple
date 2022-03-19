// Package unique implements a proxy.Server that wants to relay data to a unique target address
// unique 是 listen端, 监听一个普通的tcp端口，试图将一切流量转发到特定的预定义的地址. 并不是直接转发，而是转发到dial
// unique属于 “单目标”代理，而其它proxy.Server 一般都属于 “泛目标”代理
package unique

import (
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

const name = "unique"

func init() {
	proxy.RegisterServer(name, &ServerCreator{})
}

type ServerCreator struct{}

func (_ ServerCreator) NewServerFromURL(*url.URL) (proxy.Server, error) {
	return nil, errors.New("Not implemented")
}

func (_ ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	ta, e := netLayer.NewAddrByURL(lc.TargetAddr)
	if e != nil {
		return nil, e
	}
	s := &Server{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: lc.GetAddr()}, //监听地址，不要与TargetAddr混淆
		targetAddr:        ta,
	}
	return s, nil
}

type Server struct {
	proxy.ProxyCommonStruct

	targetAddr *netLayer.Addr
	addrStr    string
}

func NewServer() (proxy.Server, error) {
	d := &Server{}
	return d, nil
}
func (d *Server) Name() string { return name }

func (s *Server) Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error) {
	return underlay, s.targetAddr, nil
}
func (s *Server) CanFallback() bool {
	return false
}
func (s *Server) Stop() {
	// Nothing to stop or close
}
