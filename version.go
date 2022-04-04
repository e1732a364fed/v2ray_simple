/*Package main 读取配置文件，将其内容转化为 proxy.Client和 proxy.Server，然后进行代理转发.

命令行参数请使用 --help查看详情。

如果一个命令行参数无法在标准配置中进行配置，那么它就属于高级选项，或者不推荐的选项，或者正在开发中的功能.

Config Format  配置格式

一共有三种配置格式，极简模式，标准模式，兼容模式。

“极简模式”(即 verysimple mode)，入口和出口仅有一个，而且都是使用共享链接的url格式来配置.

标准模式使用toml格式。

兼容模式可以兼容v2ray现有json格式。（暂未实现）。

极简模式的理念是，配置文件的字符尽量少，尽量短小精悍;

还有个命令行模式，就是直接把极简模式的url 放到命令行参数中，比如:

	verysimple -L socks5://sfdfsaf -D direct://


Structure 本项目结构

	main -> proxy.Standard(配置文件) -> netLayer-> tlsLayer -> httpLayer -> ws -> proxy.

	main中，读取配置文件，生成 Dail、Listen 、 RoutePolicy 和 Fallback等 对象后，开始监听；

	具体调用链 是 listenSer -> handleNewIncomeConnection -> handshakeInserver_and_passToOutClient -> dialClient

	用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过ws, 然后都搞好后，传到proxy，然后就开始转发

*/
package main

import (
	"fmt"
	"runtime"

	"github.com/hahahrfool/v2ray_simple/netLayer"
)

var Version string //版本号由 Makefile 里的 BUILD_VERSION 指定

func versionStr() string {
	return fmt.Sprintf("verysimple %s, %s %s %s", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

func printVersion_simple() {
	fmt.Printf(versionStr())
	fmt.Printf("\n")
}

func printVersion() {
	const delimiter = "===============================\n"

	fmt.Printf(delimiter)
	printVersion_simple()
	fmt.Printf(delimiter)

	const desc = "A very simple implementation of V2Ray with some innovation"
	fmt.Printf(desc)
	fmt.Printf("\n")

	fmt.Printf("Support tls, grpc, websocket, quic for all protocols.\n")
	if netLayer.HasEmbedGeoip() {
		fmt.Printf("Contains embeded Geoip file\n")
	}
	fmt.Printf(delimiter)

}
