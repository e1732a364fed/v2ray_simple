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

func convertQxToVs() {
	dc := configAdapter.FromQX(cmdConvertQxToVs)
	fmt.Println(utils.GetPurgedTomlStr(proxy.StandardConf{
		Dial: []*proxy.DialConf{&dc},
	}))

}

func extractQxRemoteServers() {
	var bs []byte
	var readE error
	if strings.HasPrefix(cmdExtractQX_remoteServer, "http") {

		fmt.Printf("downloading %s\n", cmdExtractQX_remoteServer)

		resp, err := http.DefaultClient.Get(cmdExtractQX_remoteServer)

		if err != nil {
			fmt.Printf("Download failed %s\n", err.Error())
			return
		}
		defer resp.Body.Close()

		counter := &utils.DownloadPrintCounter{}

		bs, readE = io.ReadAll(io.TeeReader(resp.Body, counter))
		fmt.Printf("\n")
	} else {
		if utils.FileExist(cmdExtractQX_remoteServer) {
			path := utils.GetFilePath(cmdExtractQX_remoteServer)
			f, e := os.Open(path)
			if e != nil {
				fmt.Printf("Download failed %s\n", e.Error())
				return
			}
			bs, readE = io.ReadAll(f)
		} else {
			fmt.Printf("file not exist %s\n", cmdExtractQX_remoteServer)
			return

		}
	}

	if readE != nil {
		fmt.Printf("read failed %s\n", readE.Error())
		return
	}
	configAdapter.ExtractQxRemoteServers(string(bs))
}
