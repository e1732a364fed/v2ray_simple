package proxy

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//极简配置模式；不支持 ws/grpc
type Simple struct {
	Server_ThatListenPort_Url string                    `json:"listen"`
	Client_ThatDialRemote_Url string                    `json:"dial"`
	Route                     []*RuleConf               `json:"route" toml:"route"`
	Fallbacks                 []*httpLayer.FallbackConf `json:"fallbacks"`
	MyCountryISO_3166         string                    `toml:"mycountry" json:"mycountry"`
}

func LoadSimpleConfigFile(fileNamePath string) (*Simple, error) {

	if cf, err := os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := ioutil.ReadAll(cf)
		config := &Simple{}
		if err = json.Unmarshal(bs, config); err != nil {
			return nil, utils.NewDataErr("can not parse config file ", err, fileNamePath)
		}

		return config, nil
	} else {
		return nil, utils.NewErr("can't open config file", err)
	}

}

func LoadSimpleConfigFromStr(str string) (*Simple, error) {
	config := &Simple{}
	if err := json.Unmarshal([]byte(str), config); err != nil {
		return nil, utils.NewErr("can not parse config ", err)
	}
	return config, nil
}
