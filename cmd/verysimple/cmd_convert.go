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
)

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
