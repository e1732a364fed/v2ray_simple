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

const (
	mmdbDownloadLink = "https://cdn.jsdelivr.net/gh/Loyalsoldier/geoip@release/Country.mmdb"
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

	log.Println("Your new randomly generated uuid is : ", utils.GenerateUUIDStr())
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
	log.Println("No GeoLite2-Country.mmdb found,start downloading from " + mmdbDownloadLink)

	resp, err := http.Get(mmdbDownloadLink)

	if err != nil {
		log.Println("Download mmdb failed", err)
		return
	}
	defer resp.Body.Close()

	out, err := os.Create(netLayer.GeoipFileName)
	if err != nil {
		log.Println("Download mmdb but Can't CreateFile,", err)
		return
	}
	defer out.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Download mmdb bad status:", resp.Status)
		return
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Println("Write downloaded mmdb to file err:", err)
		return
	}
	log.Println("Download mmdb success!")

}
