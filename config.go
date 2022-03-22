package main

import (
	"flag"
	"log"
	"net/url"
	"path/filepath"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

var (
	jsonMode int
)

func init() {
	flag.IntVar(&jsonMode, "jm", 0, "json mode, 0:verysimple mode; 1: v2ray mode(not implemented yet)")
}

// set conf variable, or exit the program; 还会设置mainFallback
// 先检查configFileName是否存在，存在就尝试加载文件，否则尝试 -L参数
func loadConfig() {
	var err error

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {
			standardConf, err = proxy.LoadTomlConfFile(configFileName)
			if err != nil {

				log.Fatalln("can not load standard config file: ", err)
			}
			//log.Println("standardConf.Fallbacks: ", len(standardConf.Fallbacks))
			if len(standardConf.Fallbacks) != 0 {
				mainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)

			}
			confMode = 1
			if appConf := standardConf.App; appConf != nil {
				utils.LogLevel = appConf.LogLevel
				default_uuid = appConf.DefaultUUID
				if appConf.NoReadV {
					netLayer.UseReadv = false
				}
			}
			return
		} else {
			//默认认为所有其他后缀的都是json格式，因为有时我会用 server.json.vless 这种写法
			// 默认所有json格式的文件都为 极简模式

			simpleConf, err = proxy.LoadSimpleConfigFile(configFileName)
			if err != nil {

				log.Fatalln("can not load simple config file: ", err)
			}
			if simpleConf.Fallbacks != nil {
				mainFallback = httpLayer.NewClassicFallbackFromConfList(simpleConf.Fallbacks)
			}
			confMode = 0

			if simpleConf.Client_ThatDialRemote_Url == "" {
				simpleConf.Client_ThatDialRemote_Url = "direct://"
			}
			return
		}

	} else {
		log.Println("No Such Config File:", configFileName, ",will try using -L parameter ")
		if listenURL != "" {
			_, err = url.Parse(listenURL)
			if err != nil {
				log.Fatalln("listenURL given but invalid ", listenURL, err)

			}

			simpleConf = &proxy.Simple{
				Server_ThatListenPort_Url: listenURL,
			}

			if dialURL != "" {

				_, err = url.Parse(dialURL)
				if err != nil {
					log.Fatalln("dialURL given but invalid ", dialURL, err)

				}

				simpleConf.Client_ThatDialRemote_Url = dialURL
			}

		} else {
			log.Fatalln("no -L listen URL provided ")

		}
	}

}
