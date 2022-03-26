package main

import (
	"flag"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

var (
	cmdPrintSupportedProtocols bool
)

func init() {
	flag.BoolVar(&cmdPrintSupportedProtocols, "sp", false, "print supported protocols")

}
func mayPrintSupportedProtocols() {
	if !cmdPrintSupportedProtocols {
		return
	}
	proxy.PrintAllServerNames()
	proxy.PrintAllClientNames()
}
