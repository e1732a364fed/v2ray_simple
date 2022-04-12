package vless_test

import (
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestVLess0(t *testing.T) {
	proxy.TestTCP("vless", 0, netLayer.RandPortStr(), t)
}

func TestVLess1(t *testing.T) {
	proxy.TestTCP("vless", 1, netLayer.RandPortStr(), t)
}

func TestVLess0_udp(t *testing.T) {
	proxy.TestUDP("vless", 0, netLayer.RandPortStr(), 0, t)
}

func TestVLess1_udp(t *testing.T) {
	proxy.TestUDP("vless", 1, netLayer.RandPortStr(), 0, t)
}

func TestVLess1_udp_multi(t *testing.T) {
	proxy.TestUDP("vless", 1, netLayer.RandPortStr(), 1, t)
}
