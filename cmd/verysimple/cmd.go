package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	httpProxy "github.com/e1732a364fed/v2ray_simple/proxy/http"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"

	vs "github.com/e1732a364fed/v2ray_simple"
	"go.uber.org/zap"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//本文件下所有命令的输出统一使用 fmt 而不是 log

var (
	cmdPrintSupportedProtocols bool
	cmdPrintVer                bool

	interactive_mode bool
	download         bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&cmdPrintVer, "v", false, "print the version string then exit")
	flag.BoolVar(&download, "d", false, " automatically download required mmdb file")

}

// 运行一些 执行后立即退出程序的 命令
func runExitCommands() (atLeastOneCalled bool) {
	if cmdPrintVer {
		atLeastOneCalled = true
		printVersion_simple(os.Stdout)
	}

	if cmdPrintSupportedProtocols {
		atLeastOneCalled = true
		printSupportedProtocols()
	}

	return
}

// 在开始正式代理前, 先运行一些需要运行的命令与函数
func runPreCommands() {

	if download {
		tryDownloadMMDB()

		tryDownloadGeositeSource()
	}
}

func generateAndPrintUUID() {
	fmt.Printf("New random uuid : %s\n", utils.GenerateUUIDStr())
}

func generateRandomSSlCert() {
	const certFn = "cert.pem"
	const keyFn = "cert.key"
	if utils.FileExist(certFn) {
		utils.PrintStr(certFn)
		utils.PrintStr(" 已存在！\n")
		return
	}

	if utils.FileExist(keyFn) {
		utils.PrintStr(keyFn)
		utils.PrintStr(" 已存在！\n")
		return
	}

	err := tlsLayer.GenerateRandomCertKeyFiles(certFn, keyFn)
	if err == nil {
		utils.PrintStr("生成成功！请查看目录中的 ")
		utils.PrintStr(certFn)
		utils.PrintStr(" 和 ")
		utils.PrintStr(keyFn)
		utils.PrintStr("\n")

	} else {

		utils.PrintStr("生成失败,")
		utils.PrintStr(err.Error())
		utils.PrintStr("\n")

	}
}

func printSupportedProtocols() {
	utils.PrintStr("Support tcp/udp/unix domain socket/tls/uTls by default.\n")
	proxy.PrintAllServerNames()
	proxy.PrintAllClientNames()
	advLayer.PrintAllProtocolNames()
}

func printAllState(w io.Writer) {
	fmt.Fprintln(w, "activeConnectionCount", vs.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", vs.AllDownloadBytesSinceStart)
	fmt.Fprintln(w, "allUploadBytesSinceStart", vs.AllUploadBytesSinceStart)

	for i, s := range allServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetVSI_url(s, ""))

	}
	for i, c := range allClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetVSI_url(c, ""))
	}

}

// see https://dev.maxmind.com/geoip/geolite2-free-geolocation-data?lang=en
func tryDownloadMMDB() {
	fp := utils.GetFilePath(netLayer.GeoipFileName)

	if utils.FileExist(fp) {
		return
	}

	fmt.Printf("No %s found,start downloading from %s\n", netLayer.GeoipFileName, netLayer.MMDB_DownloadLink)

	var outClient proxy.Client

	if defaultOutClient != nil && defaultOutClient.Name() != proxy.DirectName && defaultOutClient.Name() != proxy.RejectName {
		outClient = defaultOutClient
		utils.PrintStr("trying to download mmdb through your proxy dial\n")
	} else {
		utils.PrintStr("trying to download mmdb directly\n")
	}

	var proxyUrl string
	var listener io.Closer

	if outClient != nil {

		clientEndInServer, proxyurl, err := httpProxy.SetupTmpProxyServer()
		if err != nil {
			fmt.Println("can not create clientEndInServer: ", err)
			return
		}

		listener = vs.ListenSer(clientEndInServer, outClient, nil)
		if listener != nil {
			proxyUrl = proxyurl
			defer listener.Close()
		}
	}

	_, resp, err := utils.TryDownloadWithProxyUrl(proxyUrl, netLayer.MMDB_DownloadLink)

	if err != nil {
		fmt.Printf("Download mmdb failed %s\n", err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Download mmdb got bad status: %s\n", resp.Status)
		return
	}

	out, err := os.Create(netLayer.GeoipFileName)
	if err != nil {
		fmt.Printf("Can Download mmdb but Can't Create File,%s \n", err.Error())
		return
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Write downloaded mmdb to file failed: %s\n", err.Error())
		return
	}
	utils.PrintStr("Download mmdb success!\n")

}

// 试图从自己已经配置好的节点去下载geosite源码文件, 如果没有节点则直连下载。
// 我们只需要一个dial配置即可. listen我们不使用配置文件的配置，而是自行监听一个随机端口用于http代理
func tryDownloadGeositeSource() {

	if netLayer.HasGeositeFolder() {
		return
	}

	var outClient proxy.Client

	if defaultOutClient != nil && defaultOutClient.Name() != proxy.DirectName && defaultOutClient.Name() != proxy.RejectName {
		outClient = defaultOutClient
		utils.PrintStr("trying to download geosite through your proxy dial\n")
	} else {
		utils.PrintStr("trying to download geosite directly\n")
	}

	var proxyUrl string
	var listener io.Closer

	if outClient != nil {

		clientEndInServer, proxyurl, err := httpProxy.SetupTmpProxyServer()
		if err != nil {
			fmt.Println("can not create clientEndInServer: ", err)
			return
		}

		listener = vs.ListenSer(clientEndInServer, outClient, nil)
		if listener != nil {
			proxyUrl = proxyurl
			defer listener.Close()
		}
	}

	netLayer.DownloadCommunity_DomainListFiles(proxyUrl)

}

func hotLoadDialConf(Default_uuid string, conf []*proxy.DialConf) (ok bool) {
	ok = true

	for _, d := range conf {

		if d.Uuid == "" && Default_uuid != "" {
			d.Uuid = Default_uuid
		}

		outClient, err := proxy.NewClient(d)
		if err != nil {
			if ce := utils.CanLogErr("can not create outClient: "); ce != nil {
				ce.Write(zap.Error(err))
			}
			ok = false
			continue
		}

		allClients = append(allClients, outClient)
		if tag := outClient.GetTag(); tag != "" {

			routingEnv.SetClient(tag, outClient)

		}
	}

	if defaultOutClient == nil {
		if len(allClients) > 0 {
			defaultOutClient = allClients[0]

		} else {
			defaultOutClient = vs.DirectClient
		}
	}
	return

}
func hotLoadListenConf(conf []*proxy.ListenConf) (ok bool) {
	ok = true

	if defaultOutClient == nil {
		defaultOutClient = vs.DirectClient
	}

	for i, l := range conf {
		inServer, err := proxy.NewServer(l)
		if err != nil {
			log.Println("can not create inServer: ", i, err)
			ok = false
			continue
		}
		lis := vs.ListenSer(inServer, defaultOutClient, &routingEnv)
		if lis != nil {
			listenCloserList = append(listenCloserList, lis)
			allServers = append(allServers, inServer)

		} else {
			ok = false
		}

	}

	return
}

func hotLoadDialUrl(theUrlStr string, format int) error {
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

	if !hotLoadDialConf("", []*proxy.DialConf{dc}) {
		return utils.ErrFailed
	}
	return nil

}

func hotLoadListenUrl(theUrlStr string, format int) error {
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
	if !hotLoadListenConf([]*proxy.ListenConf{lc}) {
		return utils.ErrFailed
	}
	return nil
}

func hotDeleteClient(index int) {
	if index < 0 || index >= len(allClients) {
		return
	}
	doomedClient := allClients[index]

	routingEnv.DelClient(doomedClient.GetTag())
	doomedClient.Stop()
	allClients = utils.TrimSlice(allClients, index)
}

func hotDeleteServer(index int) {
	if index < 0 || index >= len(listenCloserList) {
		return
	}

	listenCloserList[index].Close()
	allServers[index].Stop()

	allServers = utils.TrimSlice(allServers, index)
	listenCloserList = utils.TrimSlice(listenCloserList, index)
}

func loadSimpleServer() (result int, server proxy.Server) {
	var e error
	server, e = proxy.ServerFromURL(simpleConf.ListenUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create local server"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	allServers = append(allServers, server)

	if !server.CantRoute() && simpleConf.Route != nil {

		netLayer.LoadMaxmindGeoipFile("")

		//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
		routingEnv.RoutePolicy = netLayer.NewRoutePolicy()
		if simpleConf.MyCountryISO_3166 != "" {
			routingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(simpleConf.MyCountryISO_3166))

		}
	}
	return
}

func loadSimpleClient() (result int, client proxy.Client) {
	var e error
	client, e = proxy.ClientFromURL(simpleConf.DialUrl)
	if e != nil {
		if ce := utils.CanLogErr("can not create remote client"); ce != nil {
			ce.Write(zap.String("error", e.Error()))
		}
		result = -1
		return
	}

	allClients = append(allClients, client)
	return
}

func getStandardConfFromCurrentState() (sc proxy.StandardConf) {
	for i := range allClients {
		sc.Dial = append(sc.Dial, getDialConfFromCurrentState(i))

	}
	for i := range allServers {
		sc.Listen = append(sc.Listen, getListenConfFromCurrentState(i))

	}

	return
}

func getDialConfFromCurrentState(i int) (dc *proxy.DialConf) {
	c := allClients[i]
	dc = c.GetBase().DialConf

	return
}

func getListenConfFromCurrentState(i int) (lc *proxy.ListenConf) {
	c := allServers[i]
	lc = c.GetBase().ListenConf

	return
}
