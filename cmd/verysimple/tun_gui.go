//go:build gui && !notun

package main

import (
	"strings"

	"github.com/e1732a364fed/v2ray_simple/proxy/tun"
)

func init() {
	tun.AddManualRunCmdsListFunc = func(s []string) {
		theGuiTunStartCmds := s
		if multilineEntry != nil {
			entriesGroup.Show()
			multilineEntry.SetText(strings.Join(theGuiTunStartCmds, "\n"))
		}
	}
}
