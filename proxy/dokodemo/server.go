/* Package dokodemo implements a proxy.Server that wants to relay data to a dokodemo target address

dokodemo 是 dokodemo-door 协议的实现。目前不含透明代理功能。

dokodemo 是 listen端, 监听一个普通的tcp端口，试图将一切流量转发到特定的预定义的地址. 并不是直接转发，而是转发到dial。

dokodemo 属于 “单目标”代理，而其它proxy.Server 一般都属于 “泛目标”代理。
 内部实际上就是 指定了目标的 纯tcp/udp协议，属于监听协议中最简单、最纯粹的一种。

Example 应用例子

使用 dokodemo 做监听，用direct 拨号，指定一个target，那么实际上就是把 该监听的节点 与远程target间建立了一个信道;

dokodemo 每监听到一个新连接， 就会新增一条 与 target 间的信道.

所以可能比较适合 中转机的情况，

比如如果有两个服务器 A和 B， 和客户C。

我们只告诉C 服务器B 的地址，然后告诉它B使用vless协议。然后C就会用vless协议对B 拨号。

此时B我们实际配置 为用 dokodemo 监听，而不是用vless监听；然后 dokodemo 的目标 指向 服务器A 的 vless监听端口.
这样就形成了一个中转机制.

实际例子：
https://www.40huo.cn/blog/wireguard-over-vless.html

就是说，任意门把客户数据的出口、自己的入口点从本地搬到了某个代理服务器的入口，然后指定了该数据的实际远程目标；就好像数据是从代理服务器直接发出的一样

到底是哪个代理服务器，由outbound（即本作的dial）以及routing配置决定的。如果没有配置routing，那就是默认走第一个dial.
*/
package dokodemo

import (
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

const name = "dokodemo"

func init() {
	proxy.RegisterServer(name, &ServerCreator{})
}

type ServerCreator struct{}

// NewServerFromURL returns "Not implemented".
//因为 tcp:// 这种url没法轻易放在 url的query里，还需转义，所以不实用
func (_ ServerCreator) NewServerFromURL(*url.URL) (proxy.Server, error) {
	return nil, errors.New("Not implemented")
}

// use lc.TargetAddr
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
}
