/*
Package main 读取配置文件，将其内容转化为 proxy.Client和 proxy.Server，然后进行代理转发.

命令行参数请使用 --help查看详情。

如果一个命令行参数无法在标准配置中进行配置，那么它就属于高级选项，或者不推荐的选项，或者正在开发中的功能.

Config Format  配置格式

一共有三种配置格式，极简模式，标准模式，兼容模式。

“极简模式”(即 verysimple mode)，入口和出口仅有一个，而且都是使用共享链接的url格式来配置.

标准模式使用toml格式。

兼容模式可以兼容v2ray现有json格式。（暂未实现）。

极简模式的理念是，配置文件的字符尽量少，尽量短小精悍;

还有个命令行模式，就是直接把极简模式的url 放到命令行参数中，比如:

	verysimple -L socks5://sfdfsaf -D direct://


Structure 本项目结构

	main -> config -> netLayer-> tlsLayer -> httpLayer -> proxy.

	用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，然后都搞好后，传到proxy，然后就开始转发

Other - 其他

另外，本作暂时不考虑引入外界log包。依赖越少越好。

*/
package main

import (
	"flag"
	"log"
	"net/url"
	"path/filepath"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
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
