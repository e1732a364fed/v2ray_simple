/*
Package machine 定义一个 可以直接运行的有限状态机；这个机器可以直接被可执行文件或者动态库所使用.

machine把所有运行代理所需要的代码包装起来，对外像一个黑盒子。

关键点是不使用任何静态变量，所有变量都放在machine中。
*/
package machine

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

type M struct {
	ApiServerConf
	DefaultUUID string

	v2ray_simple.GlobalInfo

	DirectClient proxy.Client

	AllServers       []proxy.Server
	AllClients       []proxy.Client
	DefaultOutClient proxy.Client
	RoutingEnv       proxy.RoutingEnv

	ListenCloserList []io.Closer

	ApiServerRunning bool
	EnableApiServer  bool
}

func New() *M {
	m := new(M)
	m.AllClients = make([]proxy.Client, 0, 8)
	m.AllServers = make([]proxy.Server, 0, 8)
	m.RoutingEnv.ClientsTagMap = make(map[string]proxy.Client)
	m.DirectClient, _ = proxy.ClientFromURL(proxy.DirectURL)
	m.DefaultOutClient = m.DirectClient
	return m
}

func (m *M) Start() {
	if (m.DefaultOutClient != nil) && (len(m.AllServers) > 0) {
		for _, inServer := range m.AllServers {
			lis := v2ray_simple.ListenSer(inServer, m.DefaultOutClient, &m.RoutingEnv, &m.GlobalInfo)

			if lis != nil {
				m.ListenCloserList = append(m.ListenCloserList, lis)
			}
		}

	}

}

func (m *M) Stop() {

	for _, ser := range m.AllServers {
		if ser != nil {
			ser.Stop()
		}
	}

	for _, listener := range m.ListenCloserList {
		if listener != nil {
			listener.Close()
		}
	}

}

func (m *M) SetDefaultDirectClient() {
	m.AllClients = append(m.AllClients, v2ray_simple.DirectClient)
	m.DefaultOutClient = v2ray_simple.DirectClient

	m.RoutingEnv.SetClient("direct", v2ray_simple.DirectClient)
}

// 将fallback配置中的@转化成实际对应的server的地址
func (m *M) ParseFallbacksAtSymbol(fs []*httpLayer.FallbackConf) {
	for _, fbConf := range fs {
		if fbConf.Dest == nil {
			continue
		}
		if deststr, ok := fbConf.Dest.(string); ok && strings.HasPrefix(deststr, "@") {
			for _, s := range m.AllServers {
				if s.GetTag() == deststr[1:] {
					//log.Println("got tag fallback dest, will set to ", s.AddrStr())
					fbConf.Dest = s.AddrStr()
				}
			}

		}

	}
}

func (m *M) HasProxyRunning() bool {
	return len(m.ListenCloserList) > 0
}

func (m *M) PrintAllState(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "activeConnectionCount", m.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", m.AllDownloadBytesSinceStart)
	fmt.Fprintln(w, "allUploadBytesSinceStart", m.AllUploadBytesSinceStart)

	for i, s := range m.AllServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetVSI_url(s, ""))

	}
	for i, c := range m.AllClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetVSI_url(c, ""))
	}

}
