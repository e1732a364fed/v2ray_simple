package netLayer_test

import (
	"log"
	"net"
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

	a, e = netLayer.NewAddrByURL("unix://../../myunix.shm")
	if e != nil || a.Network != "unix" || a.Name != "../../myunix.shm" {
		t.Fail()
	}

	a, e = netLayer.NewAddrByURL("unix:///root/myunix.shm")
	if e != nil || a.Network != "unix" || a.Name != "/root/myunix.shm" {
		t.Fail()
	}

	a, e = netLayer.NewAddrByURL("tcp://::1:443")
	if e != nil || a.Network != "tcp" || a.Name != "" || !net.ParseIP("::1").Equal(a.IP) {
		t.Fail()
	}

	a, e = netLayer.NewAddrByURL("tcp://[::1]:443")
	log.Println(a, e)
	if e != nil || a.Network != "tcp" || a.Name != "" || !net.ParseIP("::1").Equal(a.IP) {
		t.Fail()
	}
}
