package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/e1732a364fed/v2ray_simple/machine"
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
	cmdPrintSupportedProtocols bool
	cmdPrintVer                bool

	download bool

	defaultApiServerConf machine.ApiServerConf
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")
	flag.BoolVar(&cmdPrintVer, "v", false, "print the version string then exit")
	flag.BoolVar(&download, "d", false, " automatically download required mmdb file")

	//apiServer stuff

	flag.BoolVar(&defaultApiServerConf.PlainHttp, "sunsafe", false, "if given, api Server will use http instead of https")

	flag.StringVar(&defaultApiServerConf.PathPrefix, "spp", "/api", "api Server Path Prefix, must start with '/' ")
	flag.StringVar(&defaultApiServerConf.AdminPass, "sap", "", "api Server admin password, but won't be used if it's empty")
	flag.StringVar(&defaultApiServerConf.Addr, "sa", "127.0.0.1:48345", "api Server listen address")
	flag.StringVar(&defaultApiServerConf.CertFile, "scert", "", "api Server tls cert file path")
	flag.StringVar(&defaultApiServerConf.KeyFile, "skey", "", "api Server tls cert key path")

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

// see https://dev.maxmind.com/geoip/geolite2-free-geolocation-data?lang=en
func tryDownloadMMDB() {
	fp := utils.GetFilePath(netLayer.GeoipFileName)

	if utils.FileExist(fp) {
		return
	}

	fmt.Printf("No %s found,start downloading from %s\n", netLayer.GeoipFileName, netLayer.MMDB_DownloadLink)

	var outClient proxy.Client

	if defaultMachine.DefaultOutClient != nil && defaultMachine.DefaultOutClient.Name() != proxy.DirectName && defaultMachine.DefaultOutClient.Name() != proxy.RejectName {
		outClient = defaultMachine.DefaultOutClient
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

	if defaultMachine.DefaultOutClient != nil && defaultMachine.DefaultOutClient.Name() != proxy.DirectName && defaultMachine.DefaultOutClient.Name() != proxy.RejectName {
		outClient = defaultMachine.DefaultOutClient
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
