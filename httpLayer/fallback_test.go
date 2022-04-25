package httpLayer_test

import (
	"net"
	"testing"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

var testf = httpLayer.FallbackConditionSet{
	Path:     "/verysimple",
	Sni:      "fake.www.verysimple.com",
	AlpnMask: 1,
}

var testMap = make(map[httpLayer.FallbackConditionSet]*netLayer.Addr)
var testMap2 = make(map[httpLayer.FallbackConditionSet]*netLayer.Addr)
var testMap3 = make(map[httpLayer.FallbackConditionSet]*netLayer.Addr)

const map2Mask = httpLayer.Fallback_sni | httpLayer.Fallback_alpn

func init() {

	testMap[httpLayer.FallbackConditionSet{
		Sni: "fake.www.verysimple.com",
	}] = &netLayer.Addr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 443,
	}

	testMap2[httpLayer.FallbackConditionSet{
		Sni:      "fake.www.verysimple.com",
		AlpnMask: 1,
	}] = &netLayer.Addr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 443,
	}

}

func TestGetFallbackAllSubsets(t *testing.T) {

	t.Log(testf.GetAllSubSets())
}

func TestTestAllSubSets(t *testing.T) {

	t.Log(testf.TestAllSubSets(httpLayer.Fallback_sni, testMap))
	t.Log(testf.TestAllSubSets(map2Mask, testMap2))
}

/*
goos: darwin
goarch: arm64

AllSubSets	       	378.2 ns/op
AllSubSets_improve1	227.7 ns/op

TestAllSubSets 肯定快不少，因为跳过了无效子集以及内存分配, 就不测了.
因为不好测，和数据本身有关，分最好情况、一般情况 和 最差情况。
*/
func BenchmarkFallbackGetAllSubSets(b *testing.B) {

	for i := 0; i < b.N; i++ {
		testf.GetAllSubSets()

	}
}
