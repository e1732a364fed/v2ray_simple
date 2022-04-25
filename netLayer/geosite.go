package netLayer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

// geosite是v2fly社区维护的，非常有用！本作以及任何其它项目都没必要另起炉灶，
// 直接使用v2fly所提供的资料即可。
//
//  然而需要注意的是，geosite是一个中国人维护的项目
// 所有网站的资料都围绕着中国人的需求产生，比如 geolocation-cn 文件，没有同类的 geolocation-us 文件.
//
// geosite数据格式可参考
// https://github.com/v2fly/v2ray-core/blob/master/app/router/routercommon/common.proto
// 或者xray的 app/router/config.proto
// 然而我们不引用任何v2ray和xray的代码, 也不使用protobuf
/*
我们只能自行读取该项目原始文件，然后生成自己的数据结构

文件格式 项目已经解释的很好了，不过使用的英文
https://github.com/v2fly/domain-list-community

# comments
include:another-file
domain:google.com @attr1 @attr2
keyword:google
regexp:www\.google\.com$
full:www.google.com

下面以中文举例方式讲解一下该geosite单个文件的内容格式

一般一行一个域名
有的行后面跟着空格 和 @和一个属性
有的行第一个字符为 #, 是注释，有的行行尾也有  # 注释

a.alimama.cn @ads

有的文件，如 amazon，有如下结构

include:amazon-ads

有的域名有如下形式

full:images-cn.ssl-images-amazon.com @cn
full:images-cn-8.ssl-images-amazon.com @cn

很显然意思是 完整匹配

有的域名连点号都没有，比如 amazon

我们要做的，首先是下载最新项目文件

获取最新版本号
curl -sL https://api.github.com/repos/v2fly/domain-list-community/releases/latest | jq -r ".tag_name"

上面输出设为 tag

下载最新源文件
wget https://github.com/v2fly/domain-list-community/archive/refs/tags/$tag.tar.xz

我们只要把这个命令行转化成go语言的形式即可

*/

var GeositeListMap = make(map[string]*GeositeList)

// v2fly经典匹配配置：
//full:v2ray.com, domain:v2ray.com, domain意思是匹配子域名,
// 如果没有冒号前缀那就是纯字符串匹配
// regexp:\.goo.*\.com$  正则表达式匹配
//geosite:cn 这种是geosite列表匹配

func IsDomainInsideGeosite(geositeName string, domain string) bool {
	geositeName = strings.ToUpper(geositeName)
	glist := GeositeListMap[geositeName]
	//log.Println("IsDomainInsideGeosite called", geositeName, len(glist))
	if glist == nil {
		return false
	}

	if _, found := glist.FullDomains[domain]; found {
		return true
	}
	if HasFullOrSubDomain(domain, MapGeositeDomainHaser(glist.Domains)) {
		return true
	}

	for _, reg := range glist.RegexDomains {
		if reg.MatchString(domain) {
			return true
		}
	}

	return false
}

type GeositeDomain struct {
	Type  string //domain, regexp, full
	Value string
	Attrs []GeositeAttr
}

type GeositeAttr struct {
	Key   string
	Value any //bool or int64
}

//GeositeList 用于内存中匹配使用
type GeositeList struct {
	//Name实际上就是v2fly Community的protobuf里的 CountryCode. Geosite本意是给一个国家的域名分类, 但是实际上功能越来越多，绝大部分Name现在实际上都是网站名称，只有 CN, GEOLOCATION-CN 的是国家名. 其它的还有很多分类名称，比如 CATEGORY-ECOMMERCE
	// 在parse过后，可以发现所有的Name都被转换成了大写字符的形式
	Name string
	//Inclusion map[string]bool //一个list可能包含另一个list, 典型的cn列表就包含了大量子表。在Parse过后，所有的Inclusion项也都被加到了Domains列表中, 所以实际上这个对于实际检索是可有可无的, v2fly的protobuf里就没有该项
	// 这个Inclusion存在的意义是防止重复添加某项，比如列表中出了两个 include相同的表，则只会被include一遍
	// 当一切都加载完毕后， Inclusion 这个map就没有存在的意义了，可以设为nil

	FullDomains  map[string]GeositeDomain
	Domains      map[string]GeositeDomain
	RegexDomains []*regexp.Regexp
}

type MapGeositeDomainHaser map[string]GeositeDomain

func (mdh MapGeositeDomainHaser) HasDomain(d string) bool {
	_, found := mdh[d]
	return found
}

//从 geosite/data 文件夹中读取所有文件并加载到 GeositeListMap 中.
//
//该 geosite/data 就是 github.com/v2fly/domain-list-community 项目的 data文件夹.
func LoadGeositeFiles() (err error) {
	dir := "geosite/data"
	dir = utils.GetFilePath(dir)
	if !utils.DirExist(dir) {
		return os.ErrNotExist
	}
	ref := make(map[string]*GeositeRawList)

	err = filepath.WalkDir(dir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		list, err := LoadGeositeFile(path)
		if err != nil {
			return err
		}
		ref[list.Name] = list
		return nil
	})
	if err != nil {
		fmt.Println("Failed: ", err)
		return
	}

	for name, list := range ref {
		pl, err := ParseGeositeList(list, ref)
		if err != nil {
			fmt.Println("Failed: ", err)
			os.Exit(1)
		}

		GeositeListMap[name] = pl.ToGeositeList()
	}
	return nil
}

// DownloadCommunity_DomainListFiles 从 v2fly/domain-list-community 下载数据文件, 并放到 geosite文件夹中。
// 如果已存在geosite文件夹，return immediately.
//
// 该函数适用于系统中没有git的情况, 如果有git我们直接 git clone就行了,而且还能不断pull进行滚动更新
func DownloadCommunity_DomainListFiles(proxyurl string) {

	dir := "geosite/data"
	dir = utils.GetFilePath(dir)
	if utils.DirExist(dir) {
		fmt.Println("geosite/data folder already exists.")
		return
	}

	var resp *http.Response
	var err error

	const requestUrl = "https://api.github.com/repos/v2fly/domain-list-community/releases/latest"

	var thehttpClient = http.DefaultClient

	if proxyurl == "" {
		resp, err = thehttpClient.Get(requestUrl)

	} else {
		url_proxy, e2 := url.Parse(proxyurl)
		if e2 != nil {
			fmt.Println("proxyurl given was wrong,", proxyurl, e2)
			return
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyURL(url_proxy),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}

		resp, err = client.Get(requestUrl)

		thehttpClient = client
	}
	if err != nil {
		fmt.Println("http get failed", err)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		fmt.Println("http read failed", err)
		return
	}

	type struct1 struct {
		Tag string `json:"tag_name"`
	}
	var s = struct1{}
	json.Unmarshal(body, &s)
	if s.Tag == "" {
		return
	}

	const downloadStr = "https://github.com/v2fly/domain-list-community/archive/refs/tags/%s.tar.gz"

	resp, err = thehttpClient.Get(fmt.Sprintf(downloadStr, s.Tag))
	if err != nil {
		fmt.Println("http get failed 2", err)
		return
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)

	if err != nil {
		fmt.Println("http read failed 2", err)
		return
	}

	fmt.Println("downloaded size", buf.Len())

	folderName, err := untarGeositeSourceFiles(&buf)

	if err != nil {
		fmt.Println("untar failed,", err)
		return
	}

	fmt.Println("download and extract success!")

	err = os.Rename(folderName, "geosite")
	if err != nil {
		fmt.Println("rename folder failed", err)
		return
	}
}

//把tar.gz内容解压出来, 并返回根文件夹名称
func untarGeositeSourceFiles(fr io.Reader) (rootFolderName string, err error) {

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		var hdr *tar.Header
		hdr, err = tr.Next()

		switch {
		case err == io.EOF:
			err = nil
			return
		case err != nil:
			return
		case hdr == nil:
			continue
		}

		dstFileDir := hdr.Name

		switch hdr.Typeflag {
		case tar.TypeDir:
			if b := utils.DirExist(dstFileDir); !b {
				if err = os.MkdirAll(dstFileDir, 0775); err != nil {
					return
				}
			}

			if rootFolderName == "" {
				rootFolderName = dstFileDir
			}

		case tar.TypeReg:

			var file *os.File
			file, err = os.OpenFile(dstFileDir, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return
			}
			_, err = io.Copy(file, tr)
			if err != nil {
				file.Close()
				return
			}

			file.Close()
		}
	}

}
