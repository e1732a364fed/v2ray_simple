//go:build !notun

package main

import (
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/utils"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/tun"
)

func init() {
	extra_preCommands = append(extra_preCommands, func() {
		if download {
			if runtime.GOOS == "windows" {
				//自动下载wintun.dll
				if utils.FileExist("wintun.dll") {
					return
				}

				if !utils.DownloadAndUnzip("wintun.zip", "https://www.wintun.net/builds/wintun-0.14.1.zip", "") {
					return
				}
				dir := ""
				switch a := runtime.GOARCH; a {
				case "386":
					dir = "x86"
				default:
					dir = a
				}
				utils.LogRunCmd("copy", "wintun\\bin\\"+dir+"\\wintun.dll", ".")
			}
		}
	})

}
