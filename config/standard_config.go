package config

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//使用toml：https://toml.io/cn/v1.0.0
//如果新协议有其他新项，可以用 map[string]interface{}
type CommonConf struct {
	Tag      string `toml:"tag"`
	Protocol string `toml:"protocol"`
	Uuid     string `toml:"uuid"`
	Host     string `toml:"host"` //ip 或域名.
	IP       string `toml:"ip"`   //给出Host后，该项可以省略; 既有Host又有ip的情况比较适合cdn
	Port     int    `toml:"port"`
	Version  int    `toml:"ver"`
	Insecure bool   `toml:"insecure"`
}

func (cc *CommonConf) GetAddr() string {
	return cc.Host + ":" + strconv.Itoa(cc.Port)
}

type ListenConf struct {
	CommonConf
	Fallback string `toml:"fallback"`
	TLSCert  string `toml:"cert"`
	TLSKey   string `toml:"key"`

	NoRoute bool `toml:"noroute"`
}

type DialConf struct {
	CommonConf
	Utls bool `toml:"utls"`
}

type RouteStruct struct {
	MyCountryISO_3166 string `json:"mycountry" toml:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流
}

type Standard struct {
	Listen []*ListenConf `toml:"listen"`
	Dial   []*DialConf   `toml:"dial"`
	Route  *RouteStruct  `toml:"route"`

	Fallbacks []*httpLayer.FallbackConf `toml:"fallback"`
}

func LoadTomlConfStr(str string) (c *Standard, err error) {
	c = &Standard{}
	_, err = toml.Decode(str, c)

	return
}

func LoadTomlConfFile(fileNamePath string) (*Standard, error) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		return LoadTomlConfStr(string(bs))
	} else {
		return nil, utils.NewErr("can't open config file", err)
	}

}
