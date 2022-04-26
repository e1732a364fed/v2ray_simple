package proxy

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/url"
	"os"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//极简配置模式；只支持json
type SimpleConf struct {
	Server_ThatListenPort_Url string                    `json:"listen"`
	Client_ThatDialRemote_Url string                    `json:"dial"`
	Route                     []*netLayer.RuleConf      `json:"route"`
	Fallbacks                 []*httpLayer.FallbackConf `json:"fallbacks"`
	MyCountryISO_3166         string                    `json:"mycountry"`
}

func LoadSimpleConfigFile(fileNamePath string) (config SimpleConf, hasError bool, E utils.ErrInErr) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		if err = json.Unmarshal(bs, &config); err != nil {
			hasError = true
			E = utils.ErrInErr{
				ErrDesc:   "can not parse config file ",
				ErrDetail: err,
				Data:      fileNamePath,
			}

		}

		return
	} else {
		hasError = true
		E = utils.ErrInErr{ErrDesc: "can't open config file", ErrDetail: err}
		return
	}

}

func LoadSimpleConfigFromStr(str string) (config SimpleConf, hasE bool, E utils.ErrInErr) {

	if err := json.Unmarshal([]byte(str), &config); err != nil {
		E = utils.ErrInErr{ErrDesc: "can not parse config ", ErrDetail: err}
		hasE = true
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
