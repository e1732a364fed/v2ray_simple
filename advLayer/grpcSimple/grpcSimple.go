/*Package grpcSimple implements grpc tunnel without importing google.golang.org/grpc.

Reference

https://github.com/Dreamacro/clash/blob/master/transport/gun/gun.go, which is under MIT license

在 clash的客户端实现 的 基础上 继续用 golang的 http2包 实现了 grpc 的 基本服务端，并改进了 原代码。

Advantages

grpcSimple包 比grpc包 小很多，替代grpc包的话，可以减小 4MB 左右的可执行文件大小。但是目前不支持 multiMode。

grpcSimple包 是很棒 很有用的 实现，而且支持  grpc的 path 的回落。

grpc虽然是定义 serviceName的，但是实际上和其他http请求一样，是用的一个path，

path就是  /serviceName/Tun

Off Topic

我们可以通过本包的代码看到，grpc实际上是 基于包的，而不是基于流的，与ws类似。

参考
https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md

*/
package grpcSimple

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
)

const grpcContentType = "application/grpc"

func init() {
	advLayer.ProtocolsMap["grpc"] = Creator{}
}

type Creator struct{}

func (Creator) PackageID() string {
	return "grpcSimple"
}

func (Creator) ProtocolName() string {
	return "grpc"
}

func (Creator) GetDefaultAlpn() (alpn string, mustUse bool) {
	// v2ray 和 xray 的grpc 因为没有自己处理tls，直接用grpc包处理的tls，且grpc包对alpn有严格要求, 要用h2.
	return "h2", true
}

func (Creator) CanHandleHeaders() bool {
	return false
}

func (Creator) IsSuper() bool {
	return false
}

func (Creator) IsMux() bool {
	return true
}

func getTunPath(sn string) string {
	var sb strings.Builder
	sb.Grow(1 + len(sn) + 4)
	sb.WriteString("/")
	sb.WriteString(sn)
	sb.WriteString("/Tun")
	return sb.String()
}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {

	var serviceName string
	if conf.Path != "" {
		serviceName = conf.Path
	} else {
		serviceName = "GunService"
	}

	c := &Client{
		Config: Config{
			ServiceName: serviceName,
			Host:        conf.Host,
		},
		path: getTunPath(conf.Path),
	}

	c.theRequest = http.Request{
		Method: http.MethodPost,
		URL: &url.URL{
			Scheme: "https",
			Host:   c.Host,
			Path:   c.path,
			// for unescape path
			//Opaque: fmt.Sprintf("//%s/%s/Tun", c.Host, c.ServiceName),
		},
		Proto:      "HTTP/2",
		ProtoMajor: 2,
		ProtoMinor: 0,
		Header:     defaultClientHeader,
	}

	return c, nil
}

func (Creator) NewServerFromConf(conf *advLayer.Conf) (advLayer.Server, error) {
	s := &Server{
		Config: Config{
			ServiceName: conf.Path,
			Host:        conf.Host,
		},
		path: getTunPath(conf.Path),
	}

	return s, nil
}
