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
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

type M struct {
	sync.RWMutex
	v2ray_simple.GlobalInfo

	AppConf

	ApiServerConf

	tomlApiServerConf ApiServerConf
	CmdApiServerConf  ApiServerConf

	standardConf proxy.StandardConf
	urlConf      proxy.UrlConf

	running          bool
	apiServerRunning bool

	DefaultOutClient proxy.Client
	routingEnv       proxy.RoutingEnv

	allServers []proxy.Server
	allClients []proxy.Client

	listenCloserList []io.Closer

	callbacks

	stateReportTicker *time.Ticker
}

func New() *M {
	m := new(M)
	m.allClients = make([]proxy.Client, 0, 8)
	m.allServers = make([]proxy.Server, 0, 8)
	m.routingEnv.ClientsTagMap = make(map[string]proxy.Client)
	directClient, _ := proxy.ClientFromURL(proxy.DirectURL)
	m.DefaultOutClient = directClient
	return m
}

// 具有 DefaultOutClient 且不是direct也不是reject; 一般表明该通向一个外界代理
func (m *M) DefaultClientUsable() bool {
	dc := m.DefaultOutClient
	if dc == nil {
		return false
	}
	n := dc.Name()
	return n != proxy.DirectName && n != proxy.RejectName
}

func (m *M) ServerCount() int {
	return len(m.allServers)
}

func (m *M) ClientCount() int {
	return len(m.allClients)
}

func (m *M) IsRunning() bool {
	return m.running
}

func (m *M) IsApiServerRunning() bool {
	return m.apiServerRunning
}

// 运行配置 以及 apiServer
func (m *M) Start() {
	if (m.DefaultOutClient != nil) && (len(m.allServers) > 0) {
		utils.Info("Starting...")
		m.Lock()
		m.running = true
		m.callToggleFallback(1)
		for _, inServer := range m.allServers {
			lis := v2ray_simple.ListenSer(inServer, m.DefaultOutClient, &m.routingEnv, &m.GlobalInfo)

			if lis != nil {
				m.listenCloserList = append(m.listenCloserList, lis)
			}
		}

		if dm := m.routingEnv.DnsMachine; dm != nil {
			dm.StartListen()
		}

		if m.stateReportTicker == nil {
			m.stateReportTicker = time.NewTicker(time.Minute * 5) //每隔五分钟输出一次目前状态

			var sw = utils.PrefixWriter{
				Writer: os.Stdout,
			}
			go func() {
				for range m.stateReportTicker.C {
					sw.Prefix = []byte(time.Now().Format("2006-01-02 15:04:05.999 "))
					sw.Write([]byte("Current state:\n"))
					m.PrintAllStateForHuman(&sw)
				}
			}()
		}
		m.Unlock()
	}

	if !m.apiServerRunning && m.EnableApiServer {
		m.TryRunApiServer()
	}

}

// 融合 CmdApiServerConf 和 TomlApiServerConf, CmdApiServerConf 的值会覆盖 TomlApiServerConf
func (m *M) SetupApiConf() {
	m.ApiServerConf = NewApiServerConf()

	m.ApiServerConf.SetNonDefault(&m.tomlApiServerConf)
	m.ApiServerConf.SetNonDefault(&m.CmdApiServerConf)
}

// Stop不会停止ApiServer
func (m *M) Stop() {
	utils.Info("Stopping...")

	m.Lock()
	m.running = false
	m.callToggleFallback(0)
	for _, ser := range m.allServers {
		if ser != nil {
			ser.Stop()
		}
	}

	for _, listener := range m.listenCloserList {
		if listener != nil {
			listener.Close()
		}
	}
	if dm := m.routingEnv.DnsMachine; dm != nil {
		dm.Stop()
	}
	if m.stateReportTicker != nil {
		m.stateReportTicker.Stop()
		m.stateReportTicker = nil
	}
	m.Unlock()
}

func (m *M) SetDefaultDirectClient() {
	m.allClients = append(m.allClients, v2ray_simple.DirectClient)
	m.DefaultOutClient = v2ray_simple.DirectClient

	m.routingEnv.SetClient("direct", v2ray_simple.DirectClient)
}

// 将fallback配置中的@转化成实际对应的server的地址
func (m *M) ParseFallbacksAtSymbol(fs []*httpLayer.FallbackConf) {
	for _, fbConf := range fs {
		if fbConf.Dest == nil {
			continue
		}
		if deststr, ok := fbConf.Dest.(string); ok && strings.HasPrefix(deststr, "@") {
			for _, s := range m.allServers {
				if s.GetTag() == deststr[1:] {

					if ce := utils.CanLogDebug("got @tag fallback dest"); ce != nil {
						ce.Write(zap.String("will set to ", s.AddrStr()), zap.String("tag", deststr[1:]))
					}
					fbConf.Dest = s.AddrStr()
				}
			}

		}

	}
}

func (m *M) HasProxyRunning() bool {
	return len(m.listenCloserList) > 0
}

func (m *M) PrintAllState(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "activeConnectionCount", m.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", m.AllDownloadBytesSinceStart)
	fmt.Fprintln(w, "allUploadBytesSinceStart", m.AllUploadBytesSinceStart)

	for i, s := range m.allServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetVSI_url(s, ""))

	}
	for i, c := range m.allClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetVSI_url(c, ""))
	}

}

// mimic PrintAllState
func (m *M) PrintAllStateForHuman(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "activeConnectionCount", m.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", humanize.Bytes(m.AllDownloadBytesSinceStart))
	fmt.Fprintln(w, "allUploadBytesSinceStart", humanize.Bytes(m.AllUploadBytesSinceStart))

	for i, s := range m.allServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetVSI_url(s, ""))

	}
	for i, c := range m.allClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetVSI_url(c, ""))
	}

}
