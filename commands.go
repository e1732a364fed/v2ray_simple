package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/quic"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

//本文件下所有命令的输出统一使用 fmt 而不是 log

var (
	cmdPrintSupportedProtocols bool
	cmdGenerateUUID            bool

	interactive_mode bool
	nodownload       bool
	cmdPrintVer      bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&cmdGenerateUUID, "gu", false, "generate a random valid uuid string")
	flag.BoolVar(&interactive_mode, "i", false, "enable interactive commandline mode")
	flag.BoolVar(&nodownload, "nd", false, "don't download any extra data files")
	flag.BoolVar(&cmdPrintVer, "v", false, "print the version string then exit")

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

	cliCmdList = append(cliCmdList, CliCmd{
		"生成随机ssl证书", func() {
			tlsLayer.GenerateRandomCertKeyFiles("yourcert.pem", "yourcert.key")
			fmt.Printf("生成成功！请查看目录中的 yourcert.pem 和 yourcert.key")
		},
	})

	cliCmdList = append(cliCmdList, CliCmd{
		"调节hy手动挡", func() {
			var arr = []string{"加速", "减速", "当前状态", "讲解"}

			Select := promptui.Select{
				Label: "请选择:",
				Items: arr,
			}

			for {
				i, result, err := Select.Run()

				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return
				}

				fmt.Printf("你选择了 %q\n", result)

				switch i {
				case 0:
					quic.TheCustomRate -= 0.1
					fmt.Printf("调好了!当前rate %f\n", quic.TheCustomRate)
				case 1:
					quic.TheCustomRate += 0.1
					fmt.Printf("调好了!当前rate %f\n", quic.TheCustomRate)
				case 2:
					fmt.Printf("当前rate %f\n", quic.TheCustomRate)
				case 3:
					fmt.Printf("rate越小越加速, rate越大越减速. 最小0.2最大1.5。实际速度倍率为 1.5/rate \n")
				}
			}
		},
	})

}

//是否是活的。如果没有监听 也没有 动态修改配置的功能，则认为当前的运行是没有灵魂的、不灵活的、腐朽的.
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

	if cmdGenerateUUID {
		generateAndPrintUUID()

	}

}

func generateAndPrintUUID() {

	fmt.Printf("New random uuid : %s\n", utils.GenerateUUIDStr())
}

func printSupportedProtocols() {

	proxy.PrintAllServerNames()
	proxy.PrintAllClientNames()
}

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
	fmt.Fprintln(w, "activeConnectionCount", activeConnectionCount)
	fmt.Fprintln(w, "allDownloadBytesSinceStart", allDownloadBytesSinceStart)

	for i, s := range allServers {
		fmt.Fprintln(w, "inServer", i, proxy.GetFullName(s), s.AddrStr())

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
	var listener net.Listener

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
		listenAddrStr := netLayer.GetRandLocalPrivateAddr()
		clientEndInServer.SetAddrStr(listenAddrStr)

		listener = listenSer(clientEndInServer, outClient, false)

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
		if defaultOutClient == nil {
			defaultOutClient = outClient
		}
		allClients = append(allClients, outClient)
	}

}
func hotLoadListenConfForRuntime(conf []*proxy.ListenConf) {

	for _, l := range conf {
		inServer, err := proxy.NewServer(l)
		if err != nil {
			log.Println("can not create inServer: ", err)
			return
		}
		listenSer(inServer, defaultOutClient, true)
		allServers = append(allServers, inServer)

	}

}
