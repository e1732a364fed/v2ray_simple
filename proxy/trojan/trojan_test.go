package trojan_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

func TestTCP(t *testing.T) {
	proxy.TestTCP("trojan", "", 0, netLayer.RandPortStr_safe(true, false), "", t)
}

func TestUDP(t *testing.T) {
	proxy.TestUDP("trojan", 0, netLayer.RandPortStr_safe(true, true), 0, t)
}
