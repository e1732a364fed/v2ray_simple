package netLayer_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

func TestUrl(t *testing.T) {
	a, e := netLayer.NewAddrByURL("udp://8.8.8.8:53")
	if e != nil || a.Network != "udp" || a.Port != 53 {
		t.Fail()
	}

	a, e = netLayer.NewAddrByURL("tls://8.8.8.8:53")
	if e != nil || a.Network != "tls" {
		t.Fail()
	}
}
