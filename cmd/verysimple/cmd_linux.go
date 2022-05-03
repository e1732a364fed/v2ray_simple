package main

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
)

func init() {

	cliCmdList = append(cliCmdList, CliCmd{
		"为tproxy设置iptables(12345端口)", func() {

			tproxy.SetIPTablesByDefault()

		},
	})
	cliCmdList = append(cliCmdList, CliCmd{
		"为tproxy移除iptables", func() {

			tproxy.CleanupIPTablesByDefault()

		},
	})
}
