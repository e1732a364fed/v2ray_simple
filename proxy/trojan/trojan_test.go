package trojan_test

import (
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestTCP(t *testing.T) {
	proxy.TestTCP("trojan", 0, netLayer.RandPortStr(true, false), t)
}

func TestUDP(t *testing.T) {
	proxy.TestUDP("trojan", 0, netLayer.RandPortStr(true, true), 0, t)
}
