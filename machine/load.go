package machine

import (
	"fmt"

	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func (m *M) LoadDialConf(conf []*proxy.DialConf) (ok bool) {
	ok = true

	for _, d := range conf {

		if d.Uuid == "" && m.DefaultUUID != "" {
			d.Uuid = m.DefaultUUID
		}

		outClient, err := proxy.NewClient(d)
		if err != nil {
			if ce := utils.CanLogErr("can not create outClient: "); ce != nil {
				ce.Write(zap.Error(err))
			}
			ok = false
			continue
		}

		m.allClients = append(m.allClients, outClient)
		if tag := outClient.GetTag(); tag != "" {

			m.routingEnv.SetClient(tag, outClient)

		}
	}

	if len(m.allClients) > 0 {
		m.DefaultOutClient = m.allClients[0]

	} else {
		m.DefaultOutClient = v2ray_simple.DirectClient
	}
	return

}

// add; when hot=true, listen the server
func (m *M) LoadListenConf(conf []*proxy.ListenConf, hot bool) (ok bool) {
	ok = true

	if m.DefaultOutClient == nil {
		m.DefaultOutClient = v2ray_simple.DirectClient
	}

	for _, l := range conf {
		if l.Uuid == "" && m.DefaultUUID != "" {
			l.Uuid = m.DefaultUUID
		}

		inServer, err := proxy.NewServer(l)
		if err != nil {

			if ce := utils.CanLogErr("Can not create listen server"); ce != nil {
				ce.Write(zap.Error(err))
			}
			ok = false
			continue
		}

		if hot {
			lis := v2ray_simple.ListenSer(inServer, m.DefaultOutClient, &m.routingEnv, &m.GlobalInfo)
			if lis != nil {
				m.listenCloserList = append(m.listenCloserList, lis)
				m.allServers = append(m.allServers, inServer)

			} else {
				ok = false
			}
		} else {
			m.allServers = append(m.allServers, inServer)
		}
	}

	return
}

// delete and stop the client
func (m *M) HotDeleteClient(index int) {
	if index < 0 || index >= len(m.allClients) {
		return
	}
	doomedClient := m.allClients[index]

	m.routingEnv.DelClient(doomedClient.GetTag())
	doomedClient.Stop()
	m.allClients = utils.TrimSlice(m.allClients, index)
}

// delete and close the server
func (m *M) HotDeleteServer(index int) {
	if index < 0 || index >= len(m.listenCloserList) {
		return
	}

	m.listenCloserList[index].Close()
	m.allServers[index].Stop()

	m.allServers = utils.TrimSlice(m.allServers, index)
	m.listenCloserList = utils.TrimSlice(m.listenCloserList, index)
}

func (m *M) loadUrlConf(hot bool) (result int) {
	var ser proxy.Server
	result, ser = m.loadUrlServer(m.urlConf)
	if result < 0 {
		return
	}
	var cli proxy.Client
	result, cli = m.loadUrlClient(m.urlConf)
	if result < 0 {
		return
	}

	if hot {
		lis := v2ray_simple.ListenSer(ser, cli, &m.routingEnv, &m.GlobalInfo)
		if lis != nil {
			m.listenCloserList = append(m.listenCloserList, lis)
		} else {
			result = -1
		}
	} else {
		m.DefaultOutClient = cli
	}

	return
}

// load failed if result <0,
func (m *M) loadUrlServer(urlConf proxy.UrlConf) (result int, server proxy.Server) {
	var e error
	server, e = proxy.ServerFromURL(urlConf.ListenUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create local server"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	m.allServers = append(m.allServers, server)

	return
}

func (m *M) loadUrlClient(urlConf proxy.UrlConf) (result int, client proxy.Client) {
	var e error
	client, e = proxy.ClientFromURL(urlConf.DialUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create remote client"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	m.allClients = append(m.allClients, client)
	return
}

// 从当前内存中的配置 导出 VSConf
func (m *M) DumpVSConf() (vc VSConf) {
	vc.StandardConf = m.DumpStandardConf()
	vc.ApiServerConf = &m.ApiServerConf
	vc.AppConf = &m.AppConf

	return
}

// 从当前内存中的配置 导出 proxy.StandardConf
func (m *M) DumpStandardConf() (sc proxy.StandardConf) {
	for i := range m.allClients {
		sc.Dial = append(sc.Dial, m.dumpDialConf(i))

	}
	for i := range m.allServers {
		sc.Listen = append(sc.Listen, m.dumpListenConf(i))

	}

	return
}

func (m *M) dumpDialConf(i int) (dc *proxy.DialConf) {
	c := m.allClients[i]
	dc = c.GetBase().DialConf

	return
}

func (m *M) dumpListenConf(i int) (lc *proxy.ListenConf) {
	c := m.allServers[i]
	lc = c.GetBase().ListenConf

	return
}

func (m *M) HotLoadDialUrl(theUrlStr string, format int) error {
	u, sn, creator, okTls, err := proxy.GetRealProtocolFromClientUrl(theUrlStr)
	if err != nil {
		fmt.Printf("parse url failed %v\n", err)
		return err
	}
	dc := &proxy.DialConf{}
	dc.Protocol = sn

	dc.TLS = okTls
	err = proxy.URLToDialConf(u, dc)
	if err != nil {
		fmt.Printf("parse url failed %v\n", err)
		return err
	}
	dc, err = creator.URLToDialConf(u, dc, format)
	if err != nil {
		fmt.Printf("parse url step 2 failed %v\n", err)
		return err
	}

	if !m.LoadDialConf([]*proxy.DialConf{dc}) {
		return utils.ErrFailed
	}
	return nil

}

func (m *M) HotLoadListenUrl(theUrlStr string, format int) error {
	u, sn, creator, okTls, err := proxy.GetRealProtocolFromServerUrl(theUrlStr)
	if err != nil {
		fmt.Printf("parse url failed %v\n", err)
		return err
	}

	lc := &proxy.ListenConf{}
	lc.Protocol = sn

	lc.TLS = okTls

	err = proxy.URLToListenConf(u, lc)
	if err != nil {
		fmt.Printf("parse url failed %v\n", err)
		return err
	}
	lc, err = creator.URLToListenConf(u, lc, format)
	if err != nil {
		fmt.Printf("parse url step 2 failed %v\n", err)
		return err
	}
	if !m.LoadListenConf([]*proxy.ListenConf{lc}, true) {
		return utils.ErrFailed
	}
	return nil
}
