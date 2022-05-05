package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

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

	interactive_mode bool
	nodownload       bool
	cmdPrintVer      bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&interactive_mode, "i", false, "enable interactive commandline mode")
	flag.BoolVar(&nodownload, "nd", false, "don't automatically download any extra data files")
	flag.BoolVar(&cmdPrintVer, "v", false, "print the version string then exit")

	//本文件 中定义的 CliCmd都是直接返回运行结果的、无需进一步交互的命令

	cliCmdList = append(cliCmdList, CliCmd{
		"生成一个随机的uuid供你参考", func() {
			generateAndPrintUUID()
		},
	})

	cliCmdList = append(cliCmdList, CliCmd{
		"下载geosite原文件", func() {
			tryDownloadGeositeSourceFromConfiguredProxy()
		},
	})

	cliCmdList = append(cliCmdList, CliCmd{
		"打印当前版本所支持的所有协议", func() {
			printSupportedProtocols()
		},
	})

	cliCmdList = append(cliCmdList, CliCmd{
		"查询当前状态", func() {
			printAllState(os.Stdout, false)
		},
	})

}

//是否可以在运行时动态修改配置。如果没有监听 apiServer 也没有 动态修改配置的功能，则当前模式不灵活，无法动态修改
func isFlexible() bool {
	return interactive_mode || apiServerRunning
}

//运行一些 执行后立即退出程序的 命令
func runExitCommands() (atLeastOneCalled bool) {
	if cmdPrintVer {
		atLeastOneCalled = true
		printVersion_simple()
	}

	if cmdPrintSupportedProtocols {
		atLeastOneCalled = true
		printSupportedProtocols()
	}

	return
}

//在开始正式代理前, 先运行一些需要运行的命令与函数
func runPreCommands() {

	if !nodownload {
		tryDownloadMMDB()
	}
}

func generateAndPrintUUID() {
	fmt.Printf("New random uuid : %s\n", utils.GenerateUUIDStr())
}

func printSupportedProtocols() {
	fmt.Printf("Support tcp/udp/tproxy/unix domain socket/tls/uTls by default.\n")
	proxy.PrintAllServerNames()
	proxy.PrintAllClientNames()
	advLayer.PrintAllProtocolNames()
}

//see https://dev.maxmind.com/geoip/geolite2-free-geolocation-data?lang=en
func tryDownloadMMDB() {

	if utils.FileExist(utils.GetFilePath(netLayer.GeoipFileName)) {
		return
	}

	const mmdbDownloadLink = "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb"

	fmt.Printf("No GeoLite2-Country.mmdb found,start downloading from%s\n", mmdbDownloadLink)

	resp, err := http.Get(mmdbDownloadLink)

	if err != nil {
		fmt.Printf("Download mmdb failed%s\n", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(netLayer.GeoipFileName)
	if err != nil {
		fmt.Printf("Download mmdb but Can't CreateFile,%s\n", err)
		return
	}
	defer out.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Download mmdb bad status:%s\n", resp.Status)
		return
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		fmt.Printf("Write downloaded mmdb to file err:%s\n", err)
		return
	}
	fmt.Printf("Download mmdb success!\n")

}

func printAllState(w io.Writer, withoutTProxy bool) {
	fmt.Fprintln(w, "activeConnectionCount", vs.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", vs.AllDownloadBytesSinceStart)
	fmt.Fprintln(w, "allUploadBytesSinceStart", vs.AllUploadBytesSinceStart)

	for i, s := range allServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetFullName(s), s.AddrStr())

	}

	if !withoutTProxy && len(tproxyList) > 0 {
		for i, tc := range tproxyList {
			fmt.Fprintln(w, "inServer", i+len(allServers), "tproxy", tc.String())
		}
	}

	for i, c := range allClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetFullName(c), c.AddrStr())
	}

}

//试图从自己已经配置好的节点去下载geosite源码文件
// 我们只需要一个dial配置即可. listen我们不使用配置文件的配置，而是自行监听一个随机端口用于http代理
func tryDownloadGeositeSourceFromConfiguredProxy() {

	var outClient proxy.Client

	if defaultOutClient != nil {
		outClient = defaultOutClient
		fmt.Println("trying to download geosite through your proxy dial")
	} else {
		fmt.Println("trying to download geosite directly")
	}

	var proxyurl string
	var listener io.Closer

	if outClient != nil {

		const tempClientConfStr = `
[[listen]]
protocol = "http"
`

		clientConf, err := proxy.LoadTomlConfStr(tempClientConfStr)
		if err != nil {
			fmt.Println("can not create LoadTomlConfStr: ", err)

			return
		}

		clientEndInServer, err := proxy.NewServer(clientConf.Listen[0])
		if err != nil {
			fmt.Println("can not create clientEndInServer: ", err)
			return
		}
		listenAddrStr := netLayer.GetRandLocalPrivateAddr(true, false)
		clientEndInServer.SetAddrStr(listenAddrStr)

		listener = vs.ListenSer(clientEndInServer, outClient, nil)

		proxyurl = "http://" + listenAddrStr

	}

	netLayer.DownloadCommunity_DomainListFiles(proxyurl)

	if listener != nil {
		listener.Close()
	}
}

func hotLoadDialConfForRuntime(Default_uuid string, conf []*proxy.DialConf) {
	for _, d := range conf {

		if d.Uuid == "" && Default_uuid != "" {
			d.Uuid = Default_uuid
		}

		outClient, err := proxy.NewClient(d)
		if err != nil {
			if ce := utils.CanLogErr("can not create outClient: "); ce != nil {
				ce.Write(zap.Error(err))
			}
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

}
func hotLoadListenConfForRuntime(conf []*proxy.ListenConf) {

	for i, l := range conf {
		inServer, err := proxy.NewServer(l)
		if err != nil {
			log.Println("can not create inServer: ", i, err)
			return
		}
		lis := vs.ListenSer(inServer, defaultOutClient, &routingEnv)
		if lis != nil {
			listenCloserArray = append(listenCloserArray, lis)
			allServers = append(allServers, inServer)

		}

	}

}

func loadSimpleServer() (result int, server proxy.Server) {
	var hase bool
	var eie utils.ErrInErr
	server, hase, eie = proxy.ServerFromURL(simpleConf.ListenUrl)
	if hase {
		if ce := utils.CanLogErr("can not create local server"); ce != nil {
			ce.Write(zap.Error(eie))
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
	var hase bool
	var eie utils.ErrInErr
	client, hase, eie = proxy.ClientFromURL(simpleConf.DialUrl)
	if hase {
		if ce := utils.CanLogErr("can not create remote client"); ce != nil {
			ce.Write(zap.Error(eie))
		}
		result = -1
		return
	}

	allClients = append(allClients, client)
	return
}
