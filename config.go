package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hahahrfool/v2ray_simple/common"
)

type Config struct {
	Server_ThatListenPort_Url string       `json:"local"`
	Client_ThatDialRemote_Url string       `json:"remote"`
	Route                     *RouteStruct `json:"route"`
}

type RouteStruct struct {
	MyCountryISO_3166 string `json:"mycountry"` //加了mycountry后，就会自动按照geoip分流
}

func loadConfig(fileName string) (*Config, error) {
	path := common.GetFilePath(fileName)
	if len(path) > 0 {
		if cf, err := os.Open(path); err == nil {
			defer cf.Close()
			bs, _ := ioutil.ReadAll(cf)
			config := &Config{}
			if err = json.Unmarshal(bs, config); err != nil {
				return nil, fmt.Errorf("can not parse config file %v, %v", fileName, err)
			}
			return config, nil
		}
	}
	return nil, fmt.Errorf("can not load config file %v", fileName)
}

func loadConfigFromStr(str string) (*Config, error) {
	config := &Config{}
	if err := json.Unmarshal([]byte(str), config); err != nil {
		return nil, common.NewErr("can not parse config ", err)
	}
	return config, nil
}
