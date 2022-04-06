package main

import (
	"errors"
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

//mainfallback, dnsMachine, routePolicy
func loadCommonComponentsFromStandardConf() {

	if len(standardConf.Fallbacks) != 0 {
		mainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)
	}

	if dnsConf := standardConf.DnsConf; dnsConf != nil {
		dnsMachine = proxy.LoadDnsMachine(dnsConf)
	}

	hasAppLevelMyCountry := (standardConf.App != nil && standardConf.App.MyCountryISO_3166 != "")

	if standardConf.Route != nil || hasAppLevelMyCountry {

		netLayer.LoadMaxmindGeoipFile("")

		routePolicy = netLayer.NewRoutePolicy()
		if hasAppLevelMyCountry {
			routePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

		}

		proxy.LoadRulesForRoutePolicy(standardConf.Route, routePolicy)
	}
}

// set conf variable, or exit the program; 还会设置mainFallback
// 先检查configFileName是否存在，存在就尝试加载文件，否则尝试 -L参数
func loadConfig() (err error) {

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {
			standardConf, err = proxy.LoadTomlConfFile(fpath)
			if err != nil {

				log.Printf("can not load standard config file: %s\n", err)
				return
			}

			confMode = 1

			//loglevel 和 noreadv这种会被 命令行覆盖的配置，需要直接在 loadConfig函数中先处理一遍
			if appConf := standardConf.App; appConf != nil {
				default_uuid = appConf.DefaultUUID

				if appConf.LogLevel != nil && !utils.IsFlagPassed("ll") {
					utils.LogLevel = *appConf.LogLevel

				}
				if appConf.NoReadV && !utils.IsFlagPassed("readv") {
					netLayer.UseReadv = false
				}
			}

			return
		} else {
			//默认认为所有其他后缀的都是json格式，因为有时会用 server.json.vless 这种写法
			// 默认所有json格式的文件都为 极简模式

			var hasE bool
			simpleConf, hasE, err = proxy.LoadSimpleConfigFile(fpath)
			if hasE {

				log.Printf("can not load simple config file: %s\n", err)
				return
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
		log.Printf("No Such Config File:%s,will try using -L parameter \n", configFileName)
		if listenURL != "" {
			_, err = url.Parse(listenURL)
			if err != nil {
				log.Printf("listenURL given but invalid %s %s\n", listenURL, err)
				return
			}

			simpleConf = proxy.Simple{
				Server_ThatListenPort_Url: listenURL,
			}

			if dialURL != "" {

				_, err = url.Parse(dialURL)
				if err != nil {
					log.Printf("dialURL given but invalid %s %s\n", dialURL, err)
					return
				}

				simpleConf.Client_ThatDialRemote_Url = dialURL
			}

		} else {
			log.Printf("no -L listen URL provided \n")
			err = errors.New("no -L listen URL provided")
			return
		}
	}
	return
}
