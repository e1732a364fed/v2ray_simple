//Package advLayer contains subpackages for Advanced Layer in VSI model.
package advLayer

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var ErrPreviousFull = errors.New("previous conn full")

var ProtocolsMap = make(map[string]Creator)

var MaxEarlyDataLen = 2048 //for ws early data

func PrintAllProtocolNames() {
	fmt.Printf("===============================\nSupported Advanced Layer protocols:\n")
	for _, v := range utils.GetMapSortedKeySlice(ProtocolsMap) {
		fmt.Print(v)
		fmt.Print("\n")
	}
}

type Creator interface {
	//NewClientFromURL(url *url.URL) (Client, error)
	NewClientFromConf(conf *Conf) (Client, error)
	NewServerFromConf(conf *Conf) (Server, error)
}

type Conf struct {
	TlsConf *tls.Config //for quic

	Host    string
	Addr    netLayer.Addr
	Path    string
	Headers map[string][]string
	IsEarly bool           //is 0-rtt; for quic and ws.
	Extra   map[string]any //quic: useHysteria, hysteria_manual, maxbyte; grpc: multiMode
}

type Client interface {
	GetPath() string
	IsMux() bool   //quic and grpc. if IsMux, then Client is a MuxClient, or it's a SingleClient
	IsEarly() bool //is 0-rtt or not.

	IsSuper() bool // quic handles transport layer dialing and tls layer handshake directly.

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

	// If IsSuper, underlay should be nil;
	//
	// If not IsSuper and underlay == nil, it will return error if it can't find any extablished connection.
	GetCommonConn(underlay net.Conn) (conn any, err error)

	DialSubConn(underlay any) (net.Conn, error)

	ProcessWhenFull(underlay any) //for quic
}

type Server interface {
	GetPath() string //for ws and grpc

	IsMux() bool //quic and grpc. if IsMux, then Server is a MuxServer, or it's a SingleServer

	IsSuper() bool //quic
}

//ws
type SingleServer interface {
	Handshake(optionalFirstBuffer *bytes.Buffer, underlay net.Conn) (net.Conn, error)
}

//grpc
type MuxServer interface {
	//non-blocking
	StartHandle(underlay net.Conn, newSubConnChan chan net.Conn)
}

//quic
type SuperMuxServer interface {
	MuxServer

	//non-blocking.  Super会直接掌控 原始链接的 监听过程, 并直接向 newSubConnChan 传递 子连接。
	StartListen() (newSubConnChan chan net.Conn, closer io.Closer)
}
