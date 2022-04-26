//go:build !linux
// +build !linux

package v2ray_simple

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//非阻塞。
func ListenTproxy(string, proxy.Client) (_ *tproxy.Machine) {
	utils.Warn("Tproxy not possible on non-linux device")
	return
}
