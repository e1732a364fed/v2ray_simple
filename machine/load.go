package machine

import (
	"fmt"

	"github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
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

		m.AllClients = append(m.AllClients, outClient)
		if tag := outClient.GetTag(); tag != "" {

			m.RoutingEnv.SetClient(tag, outClient)

		}
	}

	if m.DefaultOutClient == nil {
		if len(m.AllClients) > 0 {
			m.DefaultOutClient = m.AllClients[0]

		} else {
			m.DefaultOutClient = v2ray_simple.DirectClient
		}
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
			lis := v2ray_simple.ListenSer(inServer, m.DefaultOutClient, &m.RoutingEnv, &m.GlobalInfo)
			if lis != nil {
				m.ListenCloserList = append(m.ListenCloserList, lis)
				m.AllServers = append(m.AllServers, inServer)

			} else {
				ok = false
			}
		} else {
			m.AllServers = append(m.AllServers, inServer)
		}
	}

	return
}

// delete and stop the client
func (m *M) HotDeleteClient(index int) {
	if index < 0 || index >= len(m.AllClients) {
		return
	}
	doomedClient := m.AllClients[index]

	m.RoutingEnv.DelClient(doomedClient.GetTag())
	doomedClient.Stop()
	m.AllClients = utils.TrimSlice(m.AllClients, index)
}

// delete and close the server
func (m *M) HotDeleteServer(index int) {
	if index < 0 || index >= len(m.ListenCloserList) {
		return
	}

	m.ListenCloserList[index].Close()
	m.AllServers[index].Stop()

	m.AllServers = utils.TrimSlice(m.AllServers, index)
	m.ListenCloserList = utils.TrimSlice(m.ListenCloserList, index)
}

func (m *M) LoadSimpleServer(simpleConf proxy.SimpleConf) (result int, server proxy.Server) {
	var e error
	server, e = proxy.ServerFromURL(simpleConf.ListenUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create local server"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	m.AllServers = append(m.AllServers, server)

	if !server.CantRoute() && simpleConf.Route != nil {

		netLayer.LoadMaxmindGeoipFile("")

		//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
		m.RoutingEnv.RoutePolicy = netLayer.NewRoutePolicy()
		if simpleConf.MyCountryISO_3166 != "" {
			m.RoutingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(simpleConf.MyCountryISO_3166))

		}
	}
	return
}

func (m *M) LoadSimpleClient(simpleConf proxy.SimpleConf) (result int, client proxy.Client) {
	var e error
	client, e = proxy.ClientFromURL(simpleConf.DialUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create remote client"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	m.AllClients = append(m.AllClients, client)
	return
}

func (m *M) GetStandardConfFromCurrentState() (sc proxy.StandardConf) {
	for i := range m.AllClients {
		sc.Dial = append(sc.Dial, m.getDialConfFromCurrentState(i))

	}
	for i := range m.AllServers {
		sc.Listen = append(sc.Listen, m.getListenConfFromCurrentState(i))

	}

	return
}

func (m *M) getDialConfFromCurrentState(i int) (dc *proxy.DialConf) {
	c := m.AllClients[i]
	dc = c.GetBase().DialConf

	return
}

func (m *M) getListenConfFromCurrentState(i int) (lc *proxy.ListenConf) {
	c := m.AllServers[i]
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
