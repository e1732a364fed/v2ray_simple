package simplesocks_test

import (
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestTCP(t *testing.T) {
	proxy.TestTCP("simplesocks", 0, netLayer.RandPortStr(), t)
}

func TestUDP(t *testing.T) {
	proxy.TestUDP("simplesocks", 0, netLayer.RandPortStr(), 0, t)
}
