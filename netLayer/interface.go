package netLayer

import (
	"errors"
	"log"
	"net"
	"runtime"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var (
	// 如果机器没有ipv6地址, 就无法联通ipv6, 此时可以在dial时更快拒绝ipv6地址,
	// 避免打印过多错误输出.
	//weKnowThatWeDontHaveIPV6 bool

	ErrMachineCantConnectToIpv6 = errors.New("ErrMachineCanConnectToIpv6")
)

// 做一些网络层的资料准备工作, 可以优化本包其它函数的调用。
func PrepareInterfaces() {
	//weKnowThatWeDontHaveIPV6 = !HasIpv6Interface()
}

func GetDeviceNameIndex(idx int) string {
	intf, err := net.InterfaceByIndex(idx)
	if err != nil {
		utils.Error(err.Error())
	}
	return intf.Name
}

func HasIpv6Interface() bool {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		if ce := utils.CanLogErr("call net.InterfaceAddrs failed"); ce != nil {
			ce.Write(zap.Error(err))
		} else {
			log.Println("call net.InterfaceAddrs failed", err)

		}

		return false
	}

	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsPrivate() && !ipnet.IP.IsLinkLocalUnicast() {
			// IsLinkLocalUnicast: something starts with fe80:
			// According to godoc, If ip is not an IPv4 address, To4 returns nil.
			// This means it's ipv6
			if ipnet.IP.To4() == nil {

				if ce := utils.CanLogDebug("Has Ipv6Interface!"); ce != nil {
					ce.Write()
				} else {
					log.Println("Has Ipv6Interface!")
				}

				return true
			}
		}
	}
	return false
}

var SetSystemDNS = func(dns string) {
	if ce := utils.CanLogErr("SetSystemDNS: not implemented"); ce != nil {
		ce.Write(zap.String("platform", runtime.GOOS))
	}

	//https://www.linuxfordevices.com/tutorials/linux/change-dns-on-linux
	//linux 的 dns配置 看起来似乎不按网卡分 ，这个和 win/darwin 不同
}

var ToggleSystemProxy = func(isSocks5 bool, addr, port string, enable bool) {
	if ce := utils.CanLogErr("ToggleSystemProxy: not implemented"); ce != nil {
		ce.Write(zap.String("platform", runtime.GOOS))
	}
	//linux 中，就是设置环境变量 HTTP_PROXY, HTTPS_PROXY, 就算在ubuntu的设置界面里设置，其实也是配置的这两个变量
	// 它还同时设置了小写的 http_proxy https_proxy

	//https://linuxstans.com/how-to-set-up-proxy-ubuntu/
	//https://github.com/nityanandagohain/proxy_configuration/blob/master/proxy.py
}

var GetSystemProxyState = func(isSocks5 bool) (ok, enabled bool, addr, port string) {
	if ce := utils.CanLogErr("GetSystemProxyState: not implemented"); ce != nil {
		ce.Write(zap.String("platform", runtime.GOOS))
	}
	return
}
