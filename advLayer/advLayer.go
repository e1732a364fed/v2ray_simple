/*Package advLayer contains subpackages for Advanced Layer in VSI model.

实现包 用 ProtocolsMap 注册 Creator。
*/
package advLayer

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

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

type Creator interface {
	//NewClientFromURL(url *url.URL) (Client, error)	//todo: support url
	NewClientFromConf(conf *Conf) (Client, error)
	NewServerFromConf(conf *Conf) (Server, error)

	GetDefaultAlpn() (alpn string, mustUse bool)
	PackageID() string //unique for each package
}

type Conf struct {
	TlsConf *tls.Config //for quic

	Host    string
	Addr    netLayer.Addr
	Path    string
	Headers map[string][]string //http headers
	IsEarly bool                //is 0-rtt or not; for quic and ws.
	Extra   map[string]any      //quic: useHysteria, hysteria_manual, maxbyte; grpc: multiMode
}

type Client interface {
	IsMux() bool   //quic and grpc. if IsMux, then Client is a MuxClient, or it's a SingleClient
	IsSuper() bool // quic handles transport layer dialing and tls layer handshake directly.

	GetPath() string
	IsEarly() bool //is 0-rtt or not.

}

// ws (h1.1)
type SingleClient interface {
	Client

	//it's 0-rtt if payload is provided
	Handshake(underlay net.Conn, payload []byte) (net.Conn, error)
}

//grpc (h2) and quic (h3)
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
	IsMux() bool   //quic and grpc. if IsMux, then Server is a MuxServer, or it's a SingleServer
	IsSuper() bool //quic

	GetPath() string //for ws and grpc

	Stop()
}

//ws
type SingleServer interface {
	Handshake(optionalFirstBuffer *bytes.Buffer, underlay net.Conn) (net.Conn, error)
}

// http level fallback metadata
type FallbackMeta struct {
	FirstBuffer *bytes.Buffer
	Path        string
	Conn        net.Conn
}

//grpc
type MuxServer interface {

	//non-blocking. if fallbackChan is not nil, then it can serve for fallback feature.
	StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackChan chan FallbackMeta)
}

//quic
type SuperMuxServer interface {
	MuxServer

	//non-blocking.  Super will listen raw conn directly, and pass subStreamConn to newSubConnChan。Can stop the listening progress by closer.Close().
	StartListen() (newSubConnChan chan net.Conn, closer io.Closer)
}
