package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	vs "github.com/e1732a364fed/v2ray_simple"

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

	//commands.go 中定义的 CliCmd都是直接返回运行结果的、无需进一步交互的命令

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
			printAllState(os.Stdout)
		},
	})

}

//是否可以在运行时动态修改配置。如果没有监听 apiServer 也没有 动态修改配置的功能，则当前模式不灵活，无法动态修改
func isFlexible() bool {
	return interactive_mode || apiServerRunning
}

//在开始正式代理前, 先运行一些需要运行的命令与函数
func runPreCommands() {

	if cmdPrintSupportedProtocols {
		printSupportedProtocols()
	}

	if !nodownload {
		tryDownloadMMDB()
	}
}

func generateAndPrintUUID() {
	fmt.Printf("New random uuid : %s\n", utils.GenerateUUIDStr())
}

func printSupportedProtocols() {

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

func printAllState(w io.Writer) {
	fmt.Fprintln(w, "activeConnectionCount", vs.ActiveConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", vs.AllDownloadBytesSinceStart)
	fmt.Fprintln(w, "allUploadBytesSinceStart", vs.AllUploadBytesSinceStart)

	for i, s := range vs.AllServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetFullName(s), s.AddrStr())

	}

	for i, c := range vs.AllClients {
		fmt.Fprintln(w, "outClient", i, proxy.GetFullName(c), c.AddrStr())
	}
}

//试图从自己已经配置好的节点去下载geosite源码文件
// 我们只需要一个dial配置即可. listen我们不使用配置文件的配置，而是自行监听一个随机端口用于http代理
func tryDownloadGeositeSourceFromConfiguredProxy() {

	var outClient proxy.Client

	if vs.DefaultOutClient != nil {
		outClient = vs.DefaultOutClient
		fmt.Println("trying to download geosite through your proxy dial")
	} else {
		fmt.Println("trying to download geosite directly")
	}

	var proxyurl string
	var listener net.Listener

	if outClient != nil {

		const tempClientConfStr = `
[[listen]]
protocol = "http"
`

		clientConf, err := vs.LoadTomlConfStr(tempClientConfStr)
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

		listener = vs.ListenSer(clientEndInServer, outClient, false)

		proxyurl = "http://" + listenAddrStr

	}

	netLayer.DownloadCommunity_DomainListFiles(proxyurl)

	if listener != nil {
		listener.Close()
	}
}

func hotLoadDialConfForRuntime(conf []*proxy.DialConf) {
	for _, d := range conf {
		outClient, err := proxy.NewClient(d)
		if err != nil {
			log.Println("can not create outClient: ", err)
			return
		}
		if vs.DefaultOutClient == nil {
			vs.DefaultOutClient = outClient
		}
		vs.AllClients = append(vs.AllClients, outClient)
	}

}
func hotLoadListenConfForRuntime(conf []*proxy.ListenConf) {

	for _, l := range conf {
		inServer, err := proxy.NewServer(l)
		if err != nil {
			log.Println("can not create inServer: ", err)
			return
		}
		vs.ListenSer(inServer, vs.DefaultOutClient, true)
		vs.AllServers = append(vs.AllServers, inServer)

	}

}
