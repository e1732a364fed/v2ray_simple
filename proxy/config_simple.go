package proxy

import (
	"encoding/json"
	"io/ioutil"
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
