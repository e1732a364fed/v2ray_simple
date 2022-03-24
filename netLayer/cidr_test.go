package netLayer

import (
	"net"
	"net/netip"
	"strconv"
	"testing"

	"github.com/yl2chen/cidranger"
)

/*

go1.18
goos: darwin
goarch: arm64
Benchmark_CIDR_ranger-8         	30880119	        38.38 ns/op
Benchmark_CIDR200_ranger-8      	10963292	       107.9 ns/op
Benchmark_CIDR_netIPList-8      	341961285	         3.507 ns/op
Benchmark_CIDR60_netIPList-8    	11371914	       105.4 ns/op
Benchmark_CIDR200_netIPList-8   	 3625110	       331.4 ns/op

60个以内时，直接用 netip.Prefix 列表进行遍历更快；其他情况 cidranger 更快;

考虑到我们国别分流直接用的mmdb,而不是自己给ip段, 而如果要自定义ip段的话,很少有能自定义好几十个点，所以这个确实可以直接用列表优化一下。 不过依然是纳秒级，意义不大; 也不好说,谁知道客户端的cpu有多垃圾
*/

func Benchmark_CIDR_ranger(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	theRange := "192.168.1.0/24"
	theIPStr := "192.168.1.23"
	theIP := net.ParseIP(theIPStr)
	netRanger := cidranger.NewPCTrieRanger()
	if _, net, err := net.ParseCIDR(theRange); err == nil {
		netRanger.Insert(cidranger.NewBasicRangerEntry(*net))
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		netRanger.Contains(theIP)
	}

}

func Benchmark_CIDR200_ranger(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()
	netRanger := cidranger.NewPCTrieRanger()

	theIPStr := "192.168.1.23"
	theIP := net.ParseIP(theIPStr)

	for i := 0; i < 200; i++ {
		theRange := "192.168." + strconv.Itoa(i) + ".0/24"

		if _, net, err := net.ParseCIDR(theRange); err == nil {
			netRanger.Insert(cidranger.NewBasicRangerEntry(*net))
		}
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		netRanger.Contains(theIP)
	}

}

func Benchmark_CIDR_netIPList(b *testing.B) {

	b.StopTimer()
	b.ResetTimer()

	ipStr := "192.168.1.0/24"
	ipStr2 := "192.168.1.23"
	theIP, err := netip.ParseAddr(ipStr2)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	thelist := make([]netip.Prefix, 0, 10)

	ipnet, err := netip.ParsePrefix(ipStr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	thelist = append(thelist, ipnet)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, n := range thelist {
			if n.Contains(theIP) {
				break
			}
		}
	}

}

func Benchmark_CIDR60_netIPList(b *testing.B) {
	benchmark_CIDR_netIPList(b, 60)

}
func Benchmark_CIDR200_netIPList(b *testing.B) {
	benchmark_CIDR_netIPList(b, 200)

}

func benchmark_CIDR_netIPList(b *testing.B, num int) {

	b.StopTimer()
	b.ResetTimer()

	theIPStr := "192.168." + strconv.Itoa(num/2) + ".23"
	theIP, err := netip.ParseAddr(theIPStr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	thelist := make([]netip.Prefix, num)

	for i := 0; i < num; i++ {
		theRange := "192.168." + strconv.Itoa(i) + ".0/24"

		ipnet, err := netip.ParsePrefix(theRange)
		if err != nil {
			b.Log(err)
			b.FailNow()
		}

		thelist[i] = ipnet
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for _, n := range thelist {
			if n.Contains(theIP) {
				break
			}
		}
	}

}
