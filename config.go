/*
Package main 读取配置文件，将其内容转化为 proxy.Client和 proxy.Server，然后进行代理转发.

目前的配置文件是json格式，而且被称为“极简模式”(即 verysimple mode)，入口和出口仅有一个，而且都是使用共享链接的url格式来配置.

极简模式的理念是，配置文件的字符尽量少，尽量短小精悍;

命令行参数请使用 --help查看详情。
*/
package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/hahahrfool/v2ray_simple/utils"
)

type SimpleConfig struct {
	Server_ThatListenPort_Url string       `json:"listen"`
	Client_ThatDialRemote_Url string       `json:"dial"`
	Route                     *RouteStruct `json:"route"`
}

type RouteStruct struct {
	MyCountryISO_3166 string `json:"mycountry"` //加了mycountry后，就会自动按照geoip分流
}

// set conf variable, or exit the program
func loadConfig() {
	var err error

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {
		conf, err = loadConfigFile(configFileName)
		if err != nil {

			log.Fatalln("can not load config file: ", err)
		}
	} else {
		if listenURL != "" {
			_, err = url.Parse(listenURL)
			if err != nil {
				log.Fatalln("listenURL given but invalid ", listenURL, err)

			}

			conf = &SimpleConfig{
				Server_ThatListenPort_Url: listenURL,
			}

			if dialURL != "" {

				_, err = url.Parse(dialURL)
				if err != nil {
					log.Fatalln("dialURL given but invalid ", dialURL, err)

				}

				conf.Client_ThatDialRemote_Url = dialURL
			}

		} else {
			log.Fatalln("no listen URL provided ", configFileName)

		}
	}
	if conf.Client_ThatDialRemote_Url == "" {
		conf.Client_ThatDialRemote_Url = "direct://"
	}
}

func loadConfigFile(fileNamePath string) (*SimpleConfig, error) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		config := &SimpleConfig{}
		if err = json.Unmarshal(bs, config); err != nil {
			return nil, utils.NewDataErr("can not parse config file ", err, fileNamePath)
		}
		return config, nil
	} else {
		return nil, utils.NewErr("can't open config file", err)
	}

}

func loadConfigFromStr(str string) (*SimpleConfig, error) {
	config := &SimpleConfig{}
	if err := json.Unmarshal([]byte(str), config); err != nil {
		return nil, utils.NewErr("can not parse config ", err)
	}
	return config, nil
}
