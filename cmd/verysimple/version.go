/*Package main 读取配置文件，然后进行代理转发, 并选择性运行 交互模式和 apiServer

命令行参数请使用 --help查看详情，配置文件示例请参考 ../../examples/

如果一个命令行参数无法在标准配置中进行配置，那么它就属于高级/开发者选项，或者不推荐的选项，或者正在开发中的功能.

*/
package main

import (
	"fmt"
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
)

const delimiter = "===============================\n"

var Version string = "[version_undefined]" //版本号可由 -ldflags "-X 'main.Version=v1.x.x'" 指定, 本项目的Makefile就是用这种方式确定版本号

func versionStr() string {
	//verysimple 可以用 noquic, grpc_full tag 来选择性加载 advLayer的一些包，所以需要注明编译使用了哪些包
	var advList []string
	for _, c := range advLayer.ProtocolsMap {
		advList = append(advList, c.PackageID())
	}

	return fmt.Sprintf("verysimple %s, %s %s %s, with advLayer packages: %v", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, advList)
}

func printVersion_simple() {
	fmt.Printf(versionStr())
	fmt.Printf("\n")
}

func printVersion() {

	fmt.Printf(delimiter)
	printVersion_simple()
	fmt.Printf(delimiter)

	const desc = "A very simple implementation of V2Ray with some innovation"
	fmt.Printf(desc)
	fmt.Printf("\n")

	if netLayer.HasEmbedGeoip() {
		fmt.Printf("Contains embedded Geoip file\n")
	}
	fmt.Printf(delimiter)

}
