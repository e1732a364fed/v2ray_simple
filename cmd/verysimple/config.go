package main

import (
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	standardConf proxy.StandardConf
	appConf      *AppConf
	simpleConf   proxy.SimpleConf
)

// VS 标准toml文件格式 由 proxy.StandardConf 和 AppConf两部分组成
type VSConf struct {
	AppConf *AppConf `toml:"app"`
	proxy.StandardConf
}

// AppConf 配置App级别的配置
type AppConf struct {
	LogLevel          *int    `toml:"loglevel"` //需要为指针, 否则无法判断0到底是未给出的默认值还是 显式声明的0
	LogFile           *string `toml:"logfile"`
	DefaultUUID       string  `toml:"default_uuid"`
	MyCountryISO_3166 string  `toml:"mycountry" json:"mycountry"` //加了mycountry后，就会自动按照geoip分流,也会对顶级域名进行国别分流

	NoReadV bool `toml:"noreadv"`

	AdminPass string `toml:"admin_pass"` //用于apiServer等情况

	UDP_timeout *int `toml:"udp_timeout"`

	DialTimeoutSeconds *int `toml:"dial_timeout"`
	ReadTimeoutSeconds *int `toml:"read_timeout"`

	GeoipFile     *string `toml:"geoip_file"`
	GeositeFolder *string `toml:"geosite_folder"`
}

func setupByAppConf(ac *AppConf) {
	if ac != nil {

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
				proxy.CommonReadTimeout = time.Duration(s) * time.Second
			}
		}

		if ac.GeoipFile != nil {
			netLayer.GeoipFileName = *ac.GeoipFile
		}
		if ac.GeositeFolder != nil {
			netLayer.GeositeFolder = *ac.GeositeFolder
		}
	}
}

func LoadVSConfFromBs(bs []byte) (sc proxy.StandardConf, ac *AppConf, err error) {
	var vsConf VSConf

	bs = utils.ReplaceBytesSynonyms(bs, proxy.StandardConfBytesSynonyms)

	err = toml.Unmarshal(bs, &vsConf)

	if err != nil {
		return
	}
	sc = vsConf.StandardConf
	ac = vsConf.AppConf
	return
}

// 先检查configFileName是否存在，存在就尝试加载文件到 standardConf or simpleConf，否则尝试 listenURL, dialURL 参数.
// 若 返回的是 simpleConf, 则还可能返回 mainFallback.
func LoadConfig(configFileName, listenURL, dialURL string) (confMode int, mainFallback *httpLayer.ClassicFallback, err error) {

	fpath := utils.GetFilePath(configFileName)
	if fpath != "" {

		ext := filepath.Ext(fpath)
		if ext == ".toml" {

			if cf, err := os.Open(fpath); err == nil {
				defer cf.Close()
				bs, _ := io.ReadAll(cf)

				standardConf, appConf, err = LoadVSConfFromBs(bs)
				if err != nil {

					log.Printf("can not load standard config file: %v, \n", err)
					goto url

				}

				confMode = proxy.StandardMode

			}

		} else {

			confMode = proxy.SimpleMode
			simpleConf, mainFallback, err = proxy.LoadSimpleConf_byFile(fpath)

		}

		return

	}
url:
	if listenURL != "" {
		log.Printf("trying listenURL and dialURL \n")

		confMode = proxy.SimpleMode
		simpleConf, err = proxy.LoadSimpleConf_byUrl(listenURL, dialURL)
	} else {

		log.Println(proxy.ErrStrNoListenUrl)
		err = errors.New(proxy.ErrStrNoListenUrl)
		confMode = -1
		return
	}

	return
}
