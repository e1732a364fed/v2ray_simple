package netLayer

import (
	"log"
	"net"
	"testing"

	"github.com/hahahrfool/v2ray_simple/utils"
)

/* go test -bench "CheckMMDB_country" . -v
BenchmarkCheckMMDB_country-8   	 3631854	       315.3 ns/op

总之一次mmdb查询比map查询慢了十倍多 (见 utils/container_test.go.bak)

有必要设置一个 国别-ip 的map缓存; 不过这种纳秒级别的优化就无所谓了
*/

func BenchmarkCheckMMDB_country(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()
	LoadMaxmindGeoipFile(utils.GetFilePath("../" + GeoipFileName))

	if the_geoipdb == nil {
		log.Fatalln("err load")
	}

	theIP := net.ParseIP("1.22.233.44")

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		GetIP_ISO(theIP)
	}
}
