/*
Package main 读取配置文件，然后进行代理转发, 并选择性运行 cli/gui/apiServer.

命令行参数请使用 --help / -h 查看详情，配置文件示例请参考 ../../examples/ .

如果一个命令行参数无法在标准配置中进行配置，那么它就属于高级/开发者选项，or 不推荐的选项，or 正在开发中的功能.

# Tags

提供 noquic,notun,gui 这几个 build tag。

若 noquic给出，则不引用 advLayer/quic，否则 默认引用 advLayer/quic。
quic大概占用 2MB 大小。

若 notun给出，则不引用 proxy/tun, 否则 默认引用 proxy/tun
tun大概占用 3.5MB 大小

若gui给出，则会编译出 vs_gui 可执行文件, 会增加几MB大小.
*/
package main

import (
	"fmt"
	"io"
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"

	_ "github.com/e1732a364fed/v2ray_simple/advLayer/grpcSimple"
	_ "github.com/e1732a364fed/v2ray_simple/advLayer/ws"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/dokodemo"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/shadowsocks"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/simplesocks"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/socks5http" //该包自动引用 socks5 和 http
	_ "github.com/e1732a364fed/v2ray_simple/proxy/trojan"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vless"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vmess"
)

const (
	desc      = "A very simple app\n"
	delimiter = "===============================\n"
	weblink   = "https://github.com/e1732a364fed/v2ray_simple/"
)

var Version string = "[version_undefined]" //版本号可由 -ldflags "-X 'main.Version=v1.x.x'" 指定, 本项目的Makefile就是用这种方式确定版本号

func versionStr() string {
	//verysimple 可以用 noquic 等 tag 来选择性加载 advLayer的一些包，所以需要注明编译使用了哪些包
	var advList []string
	for _, c := range advLayer.ProtocolsMap {
		advList = append(advList, c.PackageID())
	}

	return fmt.Sprintf("verysimple %s, %s %s %s, with advLayer packages: %v \n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, advList)
}

func printVersion_simple(w io.StringWriter) {
	w.WriteString(versionStr())
}

// printVersion 返回的信息 可以唯一确定一个编译文件的 版本以及 build tags.
func printVersion(w io.StringWriter) {

	w.WriteString(delimiter)
	printVersion_simple(w)
	w.WriteString(delimiter)

	w.WriteString(desc)

	if netLayer.HasEmbedGeoip() {
		w.WriteString("Contains embedded Geoip file\n")
	}
	w.WriteString(delimiter)

}
