package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

var (
	cmdPrintSupportedProtocols bool
	cmdGenerateUUID            bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&cmdGenerateUUID, "gu", false, "generate a random valid uuid string")

}

//在开始正式代理前, 先运行一些需要运行的命令与函数
func runPreCommands() {
	mayPrintSupportedProtocols()
	tryDownloadMMDB()
	generateAndPrintUUID()
}

func generateAndPrintUUID() {
	if !cmdGenerateUUID {
		return
	}

	log.Printf("Your new randomly generated uuid is : %s\n", utils.GenerateUUIDStr())
}

func mayPrintSupportedProtocols() {
	if !cmdPrintSupportedProtocols {
		return
	}
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
