package vmess_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

func TestTCP(t *testing.T) {
	proxy.TestTCP("vmess", 0, netLayer.RandPortStr(true, false), "", t)
}

func TestTCP_none(t *testing.T) {
	proxy.TestTCP("vmess", 0, netLayer.RandPortStr(true, false), "security=none", t)
}

func TestUDP(t *testing.T) {
	proxy.TestUDP("vmess", 0, netLayer.RandPortStr(true, true), 0, t)
}
