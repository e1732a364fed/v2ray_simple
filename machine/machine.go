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

	"github.com/e1732a364fed/v2ray_simple/proxy"
)

type M struct {
	ApiServerConf

	ActiveConnectionCount      int32
	AllDownloadBytesSinceStart uint64
	AllUploadBytesSinceStart   uint64

	DirectClient proxy.Client

	AllServers       []proxy.Server
	AllClients       []proxy.Client
	DefaultOutClient proxy.Client
	RoutingEnv       proxy.RoutingEnv

	ListenCloserList []io.Closer

	Interactive_mode bool
	ApiServerRunning bool
	Gui_mode         bool
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

func (m *M) Cleanup() {

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

func (m *M) HasProxyRunning() bool {
	return len(m.ListenCloserList) > 0
}

// 是否可以在运行时动态修改配置。如果没有开启 apiServer 开关 也没有 动态修改配置的功能，则当前模式不灵活，无法动态修改
func (m *M) IsFlexible() bool {
	return m.Interactive_mode || m.EnableApiServer || m.Gui_mode
}

func (m *M) NoFuture() bool {
	return !m.HasProxyRunning() && !m.IsFlexible()
}

func (m *M) NothingRunning() bool {
	return !m.HasProxyRunning() && !(m.Interactive_mode || m.ApiServerRunning || m.Gui_mode)
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
