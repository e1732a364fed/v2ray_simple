/*Package advLayer contains subpackages for Advanced Layer in VSI model.

 */
package advLayer

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//The implementations should use ProtocolsMap to regiester their Creator.
var ProtocolsMap = make(map[string]Creator)

//为了避免黑客攻击,我们固定earlydata最大值为2048
var MaxEarlyDataLen = 2048 //for ws early data

func PrintAllProtocolNames() {
	fmt.Printf("===============================\nSupported Advanced Layer protocols:\n")
	for _, v := range utils.GetMapSortedKeySlice(ProtocolsMap) {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

//Creator represents supported features of a advLayer sub-package, and it can create New Client and Server.
type Creator interface {
	ProtocolName() string
	PackageID() string //unique for each package, sub packages in v2ray_simple don't need to apply prefix, but if you want to implement your own package, you should use full git path, like github.com/somebody/mypackage

	//NewClientFromURL(url *url.URL) (Client, error)	//todo: support url
	NewClientFromConf(conf *Conf) (Client, error)
	NewServerFromConf(conf *Conf) (Server, error)

	GetDefaultAlpn() (alpn string, mustUse bool)

	CanHandleHeaders() bool //If true, there won't be an extra http header layer during the relay progress, and the matching progress of the customized http headers will be handled inside this package.

	IsMux() bool // if IsMux, if is Client, then it is a MuxClient, or it's a SingleClient; if is Server, then it is a MuxServer, or it's a SingleServer

	IsSuper() bool // quic is a super protocol, which handles transport layer dialing and tls layer handshake directly. If IsSuper, then it's a SuperMuxServer

}

type Conf struct {
	TlsConf *tls.Config //for quic

	Host    string
	Addr    netLayer.Addr
	Path    string
	Headers *httpLayer.HeaderPreset
	IsEarly bool           //is 0-rtt or not; for quic and ws.
	Extra   map[string]any //quic: useHysteria, hysteria_manual, maxbyte; grpc: multiMode
}

type Common interface {
	Creator

	GetPath() string // for logging procedure to get the path settings. Doesn't have to be the full path, as long as different settings can be distinguished. For example, grpc's GetPath() might return its ServiceName.
}

type Client interface {
	Common

	IsEarly() bool //is 0-rtt or not.

}

//like ws (h1.1)
type SingleClient interface {
	Client

	//it's 0-rtt if payload is provided
	Handshake(underlay net.Conn, payload []byte) (net.Conn, error)
}

//like grpc (h2) and quic (h3)
type MuxClient interface {
	Client

	// If IsSuper, underlay should be nil; conn must be non nil when err==nil.
	//
	// If not IsSuper and underlay == nil, it will return error if it can't find any extablished connection.
	// Usually underlay  is tls.Conn.
	GetCommonConn(underlay net.Conn) (conn any, err error)

	//underlay is conn returned from  GetCommonConn
	DialSubConn(underlay any) (net.Conn, error)
}

type Server interface {
	Common

	Stop()
}

//like ws
type SingleServer interface {

	//如果遇到不符合握手条件但是却合法的http请求，可返回 httpLayer.FallbackMeta 和 httpLayer.ErrShouldFallback
	Handshake(underlay net.Conn) (net.Conn, error)
}

//like grpc
type MuxServer interface {

	//non-blocking. if fallbackChan is not nil, then it can serve for fallback feature.
	StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackChan chan httpLayer.FallbackMeta)
}

//like quic
type SuperMuxServer interface {
	MuxServer

	//non-blocking.  Super will listen raw conn directly, and pass subStreamConn to newSubConnChan。Can stop the listening progress by closer.Close().
	StartListen() (newSubConnChan chan net.Conn, closer io.Closer)
}
