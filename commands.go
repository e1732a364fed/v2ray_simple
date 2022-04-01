package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/tlsLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

var (
	cmdPrintSupportedProtocols bool
	cmdGenerateUUID            bool

	interactive_mode bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&cmdGenerateUUID, "gu", false, "generate a random valid uuid string")
	flag.BoolVar(&interactive_mode, "i", false, "enable interactive commandline mode")

	cliCmdList = append(cliCmdList, CliCmd{
		"生成uuid", func() {
			generateAndPrintUUID()
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
			log.Printf("生成成功！请查看目录中的 yourcert.pem 和 yourcert.key")
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

	tryDownloadMMDB()

	if cmdGenerateUUID {
		generateAndPrintUUID()

	}

}

func generateAndPrintUUID() {

	log.Printf("Your new randomly generated uuid is : %s\n", utils.GenerateUUIDStr())
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

	log.Printf("No GeoLite2-Country.mmdb found,start downloading from%s\n", mmdbDownloadLink)

	resp, err := http.Get(mmdbDownloadLink)

	if err != nil {
		log.Printf("Download mmdb failed%s\n", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(netLayer.GeoipFileName)
	if err != nil {
		log.Printf("Download mmdb but Can't CreateFile,%s\n", err)
		return
	}
	defer out.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Download mmdb bad status:%s\n", resp.Status)
		return
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Printf("Write downloaded mmdb to file err:%s\n", err)
		return
	}
	log.Printf("Download mmdb success!\n")

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
