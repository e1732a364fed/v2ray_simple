package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	httpProxy "github.com/e1732a364fed/v2ray_simple/proxy/http"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"

	vs "github.com/e1732a364fed/v2ray_simple"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//本文件下所有命令的输出统一使用 fmt 而不是 log

var (
	download bool

	//defaultApiServerConf machine.ApiServerConf

	extra_preCommands []func()

	exitCmds = []exitCmd{
		{name: "sp", desc: "print supported protocols", f: printSupportedProtocols},
		{name: "v", desc: "print the version string then exit", f: func() { printVersion_simple(os.Stdout) }},
		{name: "pifs", desc: "print all network interfaces", f: func() {
			netLayer.PrintAllInterface(os.Stdout)
		}},
	}
)

type exitCmd struct {
	enable, defaultBoolValue, isStr          bool
	name, desc, defaultStringValue, strValue string
	f                                        func()
	fs                                       func(string)
}

func init() {

	flag.BoolVar(&download, "d", false, " automatically download required mmdb file")

	//apiServer stuff

	//defaultApiServerConf.SetupFlags()
}

func initExitCmds() {
	for i, ec := range exitCmds {
		if ec.isStr {
			flag.StringVar(&exitCmds[i].strValue, ec.name, ec.defaultStringValue, ec.desc)

		} else {
			flag.BoolVar(&exitCmds[i].enable, ec.name, ec.defaultBoolValue, ec.desc)

		}
	}

}

// 运行一些 执行后立即退出程序的 命令
func runExitCommands() (atLeastOneCalled bool) {
	for _, ec := range exitCmds {
		if ec.isStr {
			if ec.strValue != "" {
				if ec.fs != nil {
					ec.fs(ec.strValue)
					atLeastOneCalled = true
				}
			}
		} else {
			if ec.enable {
				if ec.f != nil {
					ec.f()
					atLeastOneCalled = true
				}
			}
		}

	}

	return
}

func runPreCommands() {
	if len(extra_preCommands) > 0 {
		for _, f := range extra_preCommands {
			f()
		}
	}

}

// 在开始正式代理前, 先运行一些需要运行的命令与函数
func runPreCommandsAfterLoadConf() {

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
	proxy.PrintAllServerNames()
	proxy.PrintAllClientNames()
	advLayer.PrintAllProtocolNames()
	//todo: tlsLayer
}

// see https://dev.maxmind.com/geoip/geolite2-free-geolocation-data?lang=en
func tryDownloadMMDB() {
	fp := utils.GetFilePath(netLayer.GeoipFileName)

	if utils.FileExist(fp) {
		return
	}

	fmt.Printf("No %s found,start downloading from %s\n", netLayer.GeoipFileName, netLayer.MMDB_DownloadLink)

	var outClient proxy.Client

	if mainM.DefaultClientUsable() {
		outClient = mainM.DefaultOutClient
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

		listener = vs.ListenSer(clientEndInServer, outClient, nil, nil)
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

	if mainM.DefaultClientUsable() {
		outClient = mainM.DefaultOutClient
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

		listener = vs.ListenSer(clientEndInServer, outClient, nil, nil)
		if listener != nil {
			proxyUrl = proxyurl
			defer listener.Close()
		}
	}

	netLayer.DownloadCommunity_DomainListFiles(proxyUrl)

}
