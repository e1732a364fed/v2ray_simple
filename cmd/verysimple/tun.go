//go:build !notun

package main

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/utils"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/tun"
)

func init() {
	preCommands = append(preCommands, func() {
		if download {
			if runtime.GOOS == "windows" {
				//自动下载wintun.dll
				if utils.FileExist("wintun.dll") {
					return
				}

				if utils.DirExist("wintun") {

				} else {
					if !utils.DownloadAndUnzip("wintun.zip", "https://www.wintun.net/builds/wintun-0.14.1.zip", "") {
						return
					}
				}

				dir := ""
				switch a := runtime.GOARCH; a {
				case "386":
					dir = "x86"
				default:
					dir = a
				}

				oldDll, err := os.Open("wintun\\bin\\" + dir + "\\wintun.dll")
				if err != nil {
					fmt.Println("err", err)
					return
				}
				defer oldDll.Close()

				newFile, err := os.Create("wintun.dll")

				if err != nil {
					fmt.Println("err", err)
					return
				}
				defer newFile.Close()

				io.Copy(newFile, oldDll)
			}
		}
	})

}
