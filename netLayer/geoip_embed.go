//go:build embed_geoip
// +build embed_geoip

package netLayer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/oschwald/maxminddb-golang"
)

//使用 go build -tags embed_geoip 来将文件 编译进 可执行文件

//文件来自 https://dev.maxmind.com/geoip/geolite2-free-geolocation-data?lang=en
// 需要自行下载，或者到其他提供该文件的github项目下载，然后放入我们的 netLayer 文件夹中

//go:embed GeoLite2-Country.mmdb.tgz
var geoLite2Country_embed_tgz []byte

//注意，如果使用了embed版，然后又提供命令参数加载外部文件，就会内嵌版被覆盖

//将 tgz文件解压成maxmind的mmdb文件
func init() {
	embedGeoip = true

	outBuf := &bytes.Buffer{}

	gzf, err := gzip.NewReader(bytes.NewBuffer(geoLite2Country_embed_tgz))
	if err != nil {
		fmt.Println("load geoLite2Country_embed_tgz err,", err)
		os.Exit(1)
	}

	tarReader := tar.NewReader(gzf)

	header, err := tarReader.Next()

	if err != nil {

		if err == io.EOF {
			fmt.Println("load geoLite2Country_embed_tgz err,", io.EOF)
			os.Exit(1)
		}
		fmt.Println("load geoLite2Country_embed_tgz err,", err)
		os.Exit(1)
	}

	switch header.Typeflag {

	case tar.TypeReg:
		io.Copy(outBuf, tarReader)

	default:
		log.Fatal("load geoLite2Country_embed_tgz, not a simple file??", header.Name)
	}

	//这个函数应该直接接管了 我们的bytes，所以不能用 common.PutBuf 放回
	db, err := maxminddb.FromBytes(outBuf.Bytes())
	if err != nil {
		log.Fatal("maxminddb.FromBytes", err)
	}
	the_geoipdb = db

}
