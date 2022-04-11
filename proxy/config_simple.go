package proxy

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//极简配置模式；
type Simple struct {
	Server_ThatListenPort_Url string                    `json:"listen"`
	Client_ThatDialRemote_Url string                    `json:"dial"`
	Route                     []*netLayer.RuleConf      `json:"route" toml:"route"`
	Fallbacks                 []*httpLayer.FallbackConf `json:"fallbacks"`
	MyCountryISO_3166         string                    `toml:"mycountry" json:"mycountry"`
}

func LoadSimpleConfigFile(fileNamePath string) (config Simple, hasError bool, E utils.ErrInErr) {

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

func LoadSimpleConfigFromStr(str string) (config Simple, hasE bool, E utils.ErrInErr) {

	if err := json.Unmarshal([]byte(str), &config); err != nil {
		E = utils.ErrInErr{ErrDesc: "can not parse config ", ErrDetail: err}
		hasE = true
	}
	return
}
