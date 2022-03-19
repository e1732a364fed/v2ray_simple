package proxy

import (
	"io/ioutil"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//如果新协议有其他新项，可以放入 Extra.
type CommonConf struct {
	Tag      string `toml:"tag"`      //可选
	Protocol string `toml:"protocol"` //约定，如果一个Protocol尾缀去掉了's'后仍然是一个有效协议，则该协议使用了 tls
	Uuid     string `toml:"uuid"`     //一个用户的唯一标识，建议使用uuid，但也不一定
	Host     string `toml:"host"`     //ip 或域名.
	IP       string `toml:"ip"`       //给出Host后，该项可以省略; 既有Host又有ip的情况比较适合cdn
	Port     int    `toml:"port"`
	Version  int    `toml:"ver"`      //可选
	Insecure bool   `toml:"insecure"` //tls 是否安全

	IsUDP bool `toml:"udp"` //默认使用tcp监听，如果udp项给出了，则用udp监听。比如ss协议时就会用到

	Extra map[string]interface{} `toml:"extra"` //用于包含任意其它数据.虽然本包自己定义的协议肯定都是已知的，但是如果其他人使用了本包的话，那就有可能添加一些 新协议 特定的数据.
}

func (cc *CommonConf) GetAddr() string {
	return cc.Host + ":" + strconv.Itoa(cc.Port)
}

// 监听所使用的设置
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为监听地址与端口
type ListenConf struct {
	CommonConf
	Fallback string `toml:"fallback"` //回落的地址，一般可以是ip:port 或者 unix socket
	TLSCert  string `toml:"cert"`
	TLSKey   string `toml:"key"`

	NoRoute bool `toml:"noroute"` //noroute 意味着 不会进行分流，一定会被转发到默认的 dial

	TargetAddr string `toml:"target"` //若使用unique协议，则这一项会给出. 格式 tcp://127.0.0.1:443 , 必须带scheme，只能为tcp或udp

}

// 拨号所使用的设置
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为拨号地址与端口
type DialConf struct {
	CommonConf
	Utls bool `toml:"utls"`
}

type RouteStruct struct {
	MyCountryISO_3166 string `json:"mycountry" toml:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流
}

//标准配置。默认使用toml格式
// toml：https://toml.io/cn/v1.0.0
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
