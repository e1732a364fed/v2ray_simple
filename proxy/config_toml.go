package proxy

import (
	"errors"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

type AppConf struct {
	LogLevel          *int   `toml:"loglevel"` //需要为指针, 否则无法判断0到底是未给出的默认值还是 显式声明的0
	DefaultUUID       string `toml:"default_uuid"`
	MyCountryISO_3166 string `toml:"mycountry" json:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流

	NoReadV bool `toml:"noreadv"`

	AdminPass string `toml:"admin_pass"` //用于apiServer等情况

	UDP_timeout *int `toml:"udp_timeout"`
}

//标准配置。默认使用toml格式
// toml：https://toml.io/cn/
// english: https://toml.io/en/
type StandardConf struct {
	App     *AppConf          `toml:"app"`
	DnsConf *netLayer.DnsConf `toml:"dns"`

	Listen []*ListenConf `toml:"listen"`
	Dial   []*DialConf   `toml:"dial"`

	Route     []*netLayer.RuleConf      `toml:"route"`
	Fallbacks []*httpLayer.FallbackConf `toml:"fallback"`
}

func LoadTomlConfStr(str string) (c StandardConf, err error) {
	_, err = toml.Decode(str, &c)

	return
}

func LoadTomlConfFile(fileNamePath string) (StandardConf, error) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		return LoadTomlConfStr(string(bs))
	} else {
		return StandardConf{}, utils.ErrInErr{ErrDesc: "can't open config file", ErrDetail: err}
	}

}

// 先检查configFileName是否存在，存在就尝试加载文件到 standardConf 或者 simpleConf，否则尝试 -L参数
func LoadConfig(configFileName, listenURL, dialURL string) (standardConf StandardConf, simpleConf SimpleConf, confMode int, mainFallback *httpLayer.ClassicFallback, err error) {

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {
			standardConf, err = LoadTomlConfFile(fpath)
			if err != nil {

				log.Printf("can not load standard config file: %s\n", err)
				return
			}

			confMode = StandardMode

			//loglevel 和 noreadv这种会被 命令行覆盖的配置，需要直接在 loadConfig函数中先处理一遍
			if appConf := standardConf.App; appConf != nil {

				if appConf.LogLevel != nil && !utils.IsFlagGiven("ll") {
					utils.LogLevel = *appConf.LogLevel
					utils.InitLog()

				}
				if appConf.NoReadV && !utils.IsFlagGiven("readv") {
					netLayer.UseReadv = false
				}

			}

			return
		} else {

			confMode = SimpleMode
			simpleConf, mainFallback, err = loadSimpleConf_byFile(fpath)
		}

	} else {

		if listenURL != "" {
			log.Printf("No Such Config File:%s,will try using -L,-D parameter \n", configFileName)

			confMode = SimpleMode
			simpleConf, err = loadSimpleConf_byUrl(listenURL, dialURL)
		} else {
			log.Printf("no -L listen URL provided \n")
			err = errors.New("no -L listen URL provided")
			return
		}
	}
	return
}

func loadSimpleConf_byFile(fpath string) (simpleConf SimpleConf, mainFallback *httpLayer.ClassicFallback, err error) {
	//默认认为所有其他后缀的都是json格式，因为有时会用 server.json.vless 这种写法
	// 默认所有json格式的文件都为 极简模式

	var hasE bool
	simpleConf, hasE, err = LoadSimpleConfigFile(fpath)
	if hasE {

		log.Printf("can not load simple config file: %s\n", err)
		return
	}
	if simpleConf.Fallbacks != nil {
		mainFallback = httpLayer.NewClassicFallbackFromConfList(simpleConf.Fallbacks)
	}
	//ConfMode = 0

	if simpleConf.Client_ThatDialRemote_Url == "" {
		simpleConf.Client_ThatDialRemote_Url = "direct://"
	}
	return
}

func loadSimpleConf_byUrl(listenURL, dialURL string) (simpleConf SimpleConf, err error) {

	_, err = url.Parse(listenURL)
	if err != nil {
		log.Printf("listenURL given but invalid %s %s\n", listenURL, err)
		return
	}

	simpleConf = SimpleConf{
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

	return
}

type RoutingEnv struct {
	RoutePolicy  *netLayer.RoutePolicy      //used in passToOutClient
	MainFallback *httpLayer.ClassicFallback //used in checkFallback in passToOutClient
	DnsMachine   *netLayer.DNSMachine       //used in passToOutClient
}

func LoadEnvFromStandardConf(standardConf *StandardConf) (routingEnv RoutingEnv, Default_uuid string) {

	if len(standardConf.Fallbacks) != 0 {
		routingEnv.MainFallback = httpLayer.NewClassicFallbackFromConfList(standardConf.Fallbacks)
	}

	if dnsConf := standardConf.DnsConf; dnsConf != nil {
		routingEnv.DnsMachine = netLayer.LoadDnsMachine(dnsConf)
	}

	var hasAppLevelMyCountry bool

	if appConf := standardConf.App; appConf != nil {

		Default_uuid = appConf.DefaultUUID

		hasAppLevelMyCountry = appConf.MyCountryISO_3166 != ""

		if appConf.UDP_timeout != nil {
			minutes := *appConf.UDP_timeout
			if minutes > 0 {
				netLayer.UDP_timeout = time.Minute * time.Duration(minutes)
			}
		}
	}

	if standardConf.Route != nil || hasAppLevelMyCountry {

		netLayer.LoadMaxmindGeoipFile("")

		routingEnv.RoutePolicy = netLayer.NewRoutePolicy()
		if hasAppLevelMyCountry {
			routingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(standardConf.App.MyCountryISO_3166))

		}

		netLayer.LoadRulesForRoutePolicy(standardConf.Route, routingEnv.RoutePolicy)
	}

	return
}
