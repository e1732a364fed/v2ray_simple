package main

import (
	"fmt"
	"runtime"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

var Version string //版本号由 Makefile 里的 BUILD_VERSION 指定

func printVersion() {
	fmt.Printf("===============================\nverysimple %v (%v), %v %v %v\n", Version, desc, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Println("Support websocket.")
	if netLayer.HasEmbedGeoip() {
		fmt.Println("Contains embeded Geoip file")
	}

}
