//go:build !linux
// +build !linux

package main

import (
	"github.com/hahahrfool/v2ray_simple/utils"
)

func listenTproxy(addr string) {
	utils.Warn("Tproxy not possible on non-linux device")
}
