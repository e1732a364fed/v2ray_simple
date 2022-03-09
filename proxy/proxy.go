package proxy

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/hahahrfool/v2ray_simple/common"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
)

type User interface {
	GetIdentityStr() string //每个user唯一，通过比较这个string 即可 判断两个User 是否相等
}

type UserConn interface {
	io.ReadWriter
	User
	GetProtocolVersion() int
}

type ProxyCommonFuncs interface {
	AddrStr() string //地址，在server就是监听地址，在client就是拨号地址
	SetAddrStr(string)

	SetUseTLS()
	IsUseTLS() bool

	SetTLS_Server(*tlsLayer.Server)
	SetTLS_Client(*tlsLayer.Client)

	GetTLS_Server() *tlsLayer.Server
	GetTLS_Client() *tlsLayer.Client
}

func PrepareTLS_forProxyCommon(u *url.URL, isclient bool, com ProxyCommonFuncs) error {
	insecureStr := u.Query().Get("insecure")
	insecure := false
	if insecureStr != "" {
		insecure = true
	}

	if isclient {
		com.SetTLS_Client(tlsLayer.NewTlsClient(u.Host, insecure))

	} else {
		certFile := u.Query().Get("cert")
		keyFile := u.Query().Get("key")

		hostAndPort := u.Host
		sni, _, _ := net.SplitHostPort(hostAndPort)

		tlsserver, err := tlsLayer.NewServer(hostAndPort, sni, certFile, keyFile, insecure)
		if err == nil {
			com.SetTLS_Server(tlsserver)
		} else {
			return err
		}
	}
	return nil
}

type ProxyCommonStruct struct {
	Addr string
	TLS  bool

	tls_s *tlsLayer.Server
	tls_c *tlsLayer.Client
}

func (pcs *ProxyCommonStruct) InitFromUrl(u *url.URL) {
	pcs.Addr = u.Host
}

func (pcs *ProxyCommonStruct) SetTLS_Server(s *tlsLayer.Server) {
	pcs.tls_s = s
}
func (s *ProxyCommonStruct) SetTLS_Client(c *tlsLayer.Client) {
	s.tls_c = c
}

func (s *ProxyCommonStruct) GetTLS_Server() *tlsLayer.Server {
	return s.tls_s
}
func (s *ProxyCommonStruct) GetTLS_Client() *tlsLayer.Client {
	return s.tls_c
}

func (s *ProxyCommonStruct) AddrStr() string {
	return s.Addr
}
func (s *ProxyCommonStruct) SetAddrStr(a string) {
	s.Addr = a
}

func (s *ProxyCommonStruct) IsUseTLS() bool {
	return s.TLS
}
func (s *ProxyCommonStruct) SetUseTLS() {
	s.TLS = true
}

// Client is used to create connection.
type Client interface {
	ProxyCommonFuncs

	Name() string

	// Handshake的 underlay有可能传入nil，所以要求 所有的 Client 都要能够自己dial
	Handshake(underlay net.Conn, target *Addr) (io.ReadWriter, error)
}

// ClientCreator is a function to create client.
type ClientCreator func(url *url.URL) (Client, error)

var (
	clientMap = make(map[string]ClientCreator)
)

// RegisterClient is used to register a client.
func RegisterClient(name string, c ClientCreator) {
	clientMap[name] = c
}

// ClientFromURL calls the registered creator to create client.
// dialer is the default upstream dialer so cannot be nil, we can use Default when calling this function.
func ClientFromURL(s string) (Client, error) {
	u, err := url.Parse(s)
	if err != nil {

		return nil, common.NewDataErr("can not parse client url", err, s)
	}

	schemeName := strings.ToLower(u.Scheme)

	creatorFunc, ok := clientMap[schemeName]
	if ok {
		return creatorFunc(u)
	} else {

		//尝试判断是否套tls

		realScheme := strings.TrimSuffix(schemeName, "s")
		creatorFunc, ok = clientMap[realScheme]
		if ok {
			c, err := creatorFunc(u)
			if err != nil {
				return c, err
			}

			c.SetUseTLS()
			PrepareTLS_forProxyCommon(u, true, c)

			return c, err

		}

	}

	return nil, common.NewDataErr("unknown client scheme '", nil, u.Scheme)
}

// Server interface
type Server interface {
	ProxyCommonFuncs

	Name() string

	Handshake(underlay net.Conn) (io.ReadWriter, *Addr, error)
	Stop()
}

// ServerCreator is a function to create proxy server
type ServerCreator func(url *url.URL) (Server, error)

var (
	serverMap = make(map[string]ServerCreator)
)

// RegisterServer is used to register a proxy server
func RegisterServer(name string, c ServerCreator) {
	serverMap[name] = c
}

func PrintAllServerNames() {
	fmt.Println("===============================\nSupported Server protocols:")
	for v := range serverMap {
		fmt.Println(v)
	}
}

func PrintAllClientNames() {
	fmt.Println("===============================\nSupported client protocols:")

	for v := range clientMap {
		fmt.Println(v)
	}
}

// ServerFromURL calls the registered creator to create proxy servers
// dialer is the default upstream dialer so cannot be nil, we can use Default when calling this function
func ServerFromURL(s string) (Server, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, common.NewDataErr("can not parse server url ", err, s)
	}

	schemeName := strings.ToLower(u.Scheme)
	creatorFunc, ok := serverMap[schemeName]
	if ok {
		return creatorFunc(u)
	} else {
		realScheme := strings.TrimSuffix(schemeName, "s")
		creatorFunc, ok = serverMap[realScheme]
		if ok {
			server, err := creatorFunc(u)
			if err != nil {
				return server, err
			}

			server.SetUseTLS()
			PrepareTLS_forProxyCommon(u, false, server)
			return server, err

		}
	}

	return nil, common.NewDataErr("unknown server scheme '", nil, u.Scheme)
}
