package netLayer

import (
	"errors"
	"log"
	"net"

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
