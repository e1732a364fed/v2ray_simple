//go:build !linux
// +build !linux

package v2ray_simple

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//非阻塞。在非linux系统中无效。
func ListenTproxy(lc proxy.LesserConf, defaultClient proxy.Client, routePolicy *netLayer.RoutePolicy) (_ *tproxy.Machine) {
	utils.Error("Tproxy not possible on non-linux device")
	return
}
