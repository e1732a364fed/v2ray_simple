package proxy

import (
	"log"
	"net/url"
)

type UrlConf struct {
	ListenUrl string `json:"listen"`
	DialUrl   string `json:"dial"`
	//Route             []*netLayer.RuleConf      `json:"route"`
	//Fallbacks         []*httpLayer.FallbackConf `json:"fallbacks"`
	//MyCountryISO_3166 string `json:"mycountry"`
}

// load simpleConf
/*
func LoadSimpleConfigFile(fileNamePath string) (config UrlConf, err error) {

	var cf *os.File
	if cf, err = os.Open(fileNamePath); err == nil {
		defer cf.Close()
		bs, _ := io.ReadAll(cf)
		if err = json.Unmarshal(bs, &config); err != nil {
			err = utils.ErrInErr{
				ErrDesc:   "can not parse config file ",
				ErrDetail: err,
				Data:      fileNamePath,
			}

		}

		return
	} else {
		err = utils.ErrInErr{ErrDesc: "can't open config file", ErrDetail: err}
		return
	}

}


// load simpleConf
func LoadSimpleConfigFromStr(str string) (config UrlConf, hasE bool, E utils.ErrInErr) {

	if err := json.Unmarshal([]byte(str), &config); err != nil {
		E = utils.ErrInErr{ErrDesc: "can not parse config ", ErrDetail: err}
		hasE = true
	}
	return
}
*/

/*
// load simpleConf and fallbacks
func LoadSimpleConf_byFile(fpath string) (simpleConf SimpleConf, err error) {
	//默认认为所有其他后缀的都是json格式，因为有时会用 server.json.vless 这种写法
	// 默认所有json格式的文件都为 极简模式

	simpleConf, err = LoadSimpleConfigFile(fpath)
	if err != nil {

		log.Printf("can not load simple config file: %s\n", err.Error())
		return
	} else {
		err = nil
	}
	// if simpleConf.Fallbacks != nil {
	// 	mainFallback = httpLayer.NewClassicFallbackFromConfList(simpleConf.Fallbacks)
	// }

	if simpleConf.DialUrl == "" {

		simpleConf.DialUrl = DirectURL
	}
	return
}

*/

// listenURL 不可为空。dialURL如果为空，会自动被设为 DirectURL
func LoadUrlConf(listenURL, dialURL string) (simpleConf UrlConf, err error) {

	if dialURL == "" {
		dialURL = DirectURL
	}

	_, err = url.Parse(listenURL)
	if err != nil {
		log.Printf("listenURL given but invalid %s %s\n", listenURL, err.Error())
		return
	}

	simpleConf = UrlConf{
		ListenUrl: listenURL,
	}

	_, err = url.Parse(dialURL)
	if err != nil {
		log.Printf("dialURL given but invalid %s %s\n", dialURL, err.Error())
		return
	}

	simpleConf.DialUrl = dialURL

	return
}
