package main

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/machine"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	standardConf proxy.StandardConf
	appConf      *machine.AppConf
)

// 先检查configFileName是否存在，存在就尝试加载文件到 standardConf or simpleConf，否则尝试 listenURL, dialURL 参数.
// 若 返回的是 simpleConf, 则还可能返回 mainFallback.
func LoadConfig(configFileName, listenURL, dialURL string) (confMode int, simpleConf proxy.SimpleConf, mainFallback *httpLayer.ClassicFallback, err error) {

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {

			if cf, err := os.Open(fpath); err == nil {
				defer cf.Close()
				bs, _ := io.ReadAll(cf)

				standardConf, appConf, err = machine.LoadVSConfFromBs(bs)
				if err != nil {

					log.Printf("can not load standard config file: %v, \n", err)
					goto url

				}

				confMode = proxy.StandardMode

			}

		} else {

			confMode = proxy.SimpleMode
			simpleConf, mainFallback, err = proxy.LoadSimpleConf_byFile(fpath)

		}

		return

	}
url:
	if listenURL != "" {
		log.Printf("trying listenURL and dialURL \n")

		confMode = proxy.SimpleMode
		simpleConf, err = proxy.LoadSimpleConf_byUrl(listenURL, dialURL)
	} else {

		log.Println(proxy.ErrStrNoListenUrl)
		err = errors.New(proxy.ErrStrNoListenUrl)
		confMode = -1
		return
	}

	return
}
