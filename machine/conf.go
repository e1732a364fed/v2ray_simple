package machine

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// VS 标准toml文件格式 由 proxy.StandardConf , ApiServerConf, AppConf 3部分组成
type VSConf struct {
	AppConf       *AppConf       `toml:"app"`
	ApiServerConf *ApiServerConf `toml:"apiServer"`
	proxy.StandardConf
}

// AppConf 配置App级别的配置
type AppConf struct {
	LogLevel          *int    `toml:"loglevel"` //需要为指针, 否则无法判断0到底是未给出的默认值还是 显式声明的0
	LogFile           *string `toml:"logfile"`
	DefaultUUID       string  `toml:"default_uuid"`
	MyCountryISO_3166 string  `toml:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流

	NoReadV bool `toml:"noreadv"`

	UDP_timeout *int `toml:"udp_timeout"`

	DialTimeoutSeconds *int `toml:"dial_timeout"`
	ReadTimeoutSeconds *int `toml:"read_timeout"`

	GeoipFile     *string `toml:"geoip_file"`
	GeositeFolder *string `toml:"geosite_folder"`
}

func LoadVSConfFromBs(bs []byte) (vsConf VSConf, err error) {

	bs = utils.ReplaceBytesSynonyms(bs, proxy.StandardConfBytesSynonyms)

	err = toml.Unmarshal(bs, &vsConf)

	if err != nil {
		return
	}
	return
}

func GetAppConfByCurrentState() (ac AppConf) {
	lfn := utils.LogOutFileName
	if lfn != "" {
		ac.LogFile = &lfn
	}
	if ll := utils.LogLevel; ll != utils.DefaultLL {
		ac.LogLevel = &ll
	}
	if !netLayer.UseReadv {
		ac.NoReadV = true
	}
	if netLayer.UDP_timeout != netLayer.DefaultUDP_timeout {
		to := int(netLayer.UDP_timeout / time.Minute)
		ac.UDP_timeout = &to
	}
	// if netLayer.UDP_fullcone_timeout != netLayer.DefaultUDP_fullcone_timeout {

	// }

	if netLayer.DialTimeout != netLayer.DefaultDialTimeout {
		to := int(netLayer.DialTimeout / time.Second)
		ac.DialTimeoutSeconds = &to
	}

	if netLayer.CommonReadTimeout != netLayer.DefaultCommonReadTimeout {
		to := int(netLayer.CommonReadTimeout / time.Second)
		ac.ReadTimeoutSeconds = &to
	}
	return
}

func (ac *AppConf) Setup() {
	if ac == nil {
		return
	}

	if ac.LogFile != nil && utils.GivenFlags["lf"] == nil {
		utils.LogOutFileName = *ac.LogFile

	}

	if ac.LogLevel != nil && utils.GivenFlags["ll"] == nil {
		utils.LogLevel = *ac.LogLevel

	}
	if ac.NoReadV && utils.GivenFlags["readv"] == nil {
		netLayer.UseReadv = false
	}

	if ac.UDP_timeout != nil {

		if minutes := *ac.UDP_timeout; minutes > 0 {
			netLayer.UDP_timeout = time.Minute * time.Duration(minutes)
		}
	}

	if ac.DialTimeoutSeconds != nil {
		if s := *ac.DialTimeoutSeconds; s > 0 {
			netLayer.DialTimeout = time.Duration(s) * time.Second

		}
	}

	if ac.ReadTimeoutSeconds != nil {
		if s := *ac.ReadTimeoutSeconds; s > 0 {
			netLayer.CommonReadTimeout = time.Duration(s) * time.Second
		}
	}

	if ac.GeoipFile != nil {
		netLayer.GeoipFileName = *ac.GeoipFile
	}
	if ac.GeositeFolder != nil {
		netLayer.GeositeFolder = *ac.GeositeFolder
	}
}

func (m *M) LoadConfigByTomlBytes(bs []byte) (err error) {
	//var ac *AppConf
	var vsConf VSConf
	vsConf, err = LoadVSConfFromBs(bs)

	if err != nil {
		log.Printf("can not load standard config file: %v, \n", err)
		return err
	}
	m.standardConf = vsConf.StandardConf

	if vsConf.AppConf != nil {
		m.AppConf = *vsConf.AppConf

		m.AppConf.Setup()
	}
	if vsConf.ApiServerConf != nil {
		m.tomlApiServerConf = *vsConf.ApiServerConf
	}

	return nil
}

// 先检查configFileName是否存在，存在就尝试加载文件到 standardConf , 否则尝试通过 listenURL, dialURL 参数 创建urlConf. 若使用url, 自动加载进机器; 若为toml, 需要手动调用 SetupListenAndRoute 和 SetupDial
func (m *M) LoadConfig(configFileName, listenURL, dialURL string) (confMode int, err error) {

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {

			if cf, err := os.Open(fpath); err == nil {
				defer cf.Close()
				bs, _ := io.ReadAll(cf)

				err = m.LoadConfigByTomlBytes(bs)

				if err != nil {
					goto url
				}

				confMode = proxy.StandardMode
			}

		} else {
			return -1, errors.New("file passed in but no .toml suffix")
		}

		return

	}
url:
	if listenURL != "" {
		log.Printf("trying listenURL and dialURL \n")

		confMode = proxy.UrlMode
		m.urlConf, err = proxy.LoadUrlConf(listenURL, dialURL)
		if err == nil {
			r := m.loadUrlConf(false)
			if r < 0 {
				return r, errors.New("load url conf failed")
			}
		}
	} else {

		log.Println(proxy.ErrStrNoListenUrl)
		err = errors.New(proxy.ErrStrNoListenUrl)
		confMode = -1
		return
	}

	return
}

func (m *M) SetupListenAndRoute() {

	myCountryISO_3166 := m.MyCountryISO_3166

	if len(m.standardConf.Listen) < 1 {
		utils.Warn("no listen in config settings")
		return
	}

	m.LoadListenConf(m.standardConf.Listen, false)

	if len(m.standardConf.Fallbacks) > 0 {
		m.ParseFallbacksAtSymbol(m.standardConf.Fallbacks)
	}

	m.routingEnv = proxy.LoadEnvFromStandardConf(&m.standardConf, myCountryISO_3166)

}
func (m *M) SetupDial() {
	if len(m.standardConf.Dial) < 1 && m.DefaultOutClient == nil {
		utils.Warn("no dial in config settings, will add 'direct'")

		m.SetDefaultDirectClient()

		return
	}

	m.LoadDialConf(m.standardConf.Dial)
}
func (m *M) LoadStandardConf() {
	if len(m.standardConf.Dial) > 0 {
		m.LoadDialConf(m.standardConf.Dial)

	}

	if len(m.standardConf.Listen) > 0 {
		m.LoadListenConf(m.standardConf.Listen, true)

	}
}
