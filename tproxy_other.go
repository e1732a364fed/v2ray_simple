//go:build !linux
// +build !linux

package main

import (
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func listenTproxy(addr string) {
	utils.Warn("Tproxy not possible on non-linux device")
}
