//go:build !noutils

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/configAdapter"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/mdp/qrterminal"
)

// 本文件存放一些与vs核心功能无关，但是比较有用的工具命令
func init() {
	extraExitCmds := []exitCmd{
		{name: "gu", desc: "automatically generate a uuid for you", f: generateAndPrintUUID},
		{name: "gc", desc: "automatically generate random certificate for you", f: generateRandomSSlCert},

		{name: "cvqxtvs", isStr: true, desc: "if given, convert qx server config string to vs toml config", fs: convertQxToVs},
		{name: "eqxrs", isStr: true, desc: "if given, automatically extract remote servers from quantumultX config for you", fs: extractQxRemoteServers},

		{name: "qr", isStr: true, desc: "show qrcode in terminal for given string", fs: func(str string) {

			config := qrterminal.Config{ //与直接调用 GenerateHalfBlock 的区别是, 取消了QuiteZone
				HalfBlocks:     true,
				Level:          qrterminal.M,
				Writer:         os.Stdout,
				BlackChar:      qrterminal.BLACK_BLACK,
				WhiteChar:      qrterminal.WHITE_WHITE,
				BlackWhiteChar: qrterminal.BLACK_WHITE,
				WhiteBlackChar: qrterminal.WHITE_BLACK,
			}
			qrterminal.GenerateWithConfig(str, config)
		}},
		// {name: "test", desc: "test", f: func() {
		// 	// utils.InitLog("")
		// 	// dns := netLayer.GetSystemDNS()
		// 	// log.Println(len(dns), dns)
		// }},
	}

	exitCmds = append(exitCmds, extraExitCmds...)
}

func convertQxToVs(str string) {
	dc := configAdapter.FromQX(str)

	gstr, e := utils.GetPurgedTomlStr(proxy.StandardConf{
		Dial: []*proxy.DialConf{&dc},
	})

	if e != nil {
		fmt.Println(e.Error())
	} else {
		fmt.Println(gstr)
	}

}

func extractQxRemoteServers(str string) {
	var bs []byte
	var readE error
	if strings.HasPrefix(str, "http") {

		fmt.Printf("downloading %s\n", str)

		resp, err := http.DefaultClient.Get(str)

		if err != nil {
			fmt.Printf("Download failed %s\n", err.Error())
			return
		}
		defer resp.Body.Close()

		counter := &utils.DownloadPrintCounter{}

		bs, readE = io.ReadAll(io.TeeReader(resp.Body, counter))
		fmt.Printf("\n")
	} else {
		if utils.FileExist(str) {
			path := utils.GetFilePath(str)
			f, e := os.Open(path)
			if e != nil {
				fmt.Printf("Download failed %s\n", e.Error())
				return
			}
			bs, readE = io.ReadAll(f)
		} else {
			fmt.Printf("file not exist %s\n", str)
			return

		}
	}

	if readE != nil {
		fmt.Printf("read failed %s\n", readE.Error())
		return
	}
	configAdapter.ExtractQxRemoteServers(string(bs))
}
