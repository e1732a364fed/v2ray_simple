package tun

import (
	"errors"
	"net"
	"os/exec"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/net/route"
)

/*
我们的auto route使用纯命令行方式。

sing-box 使用了另一种系统级别的方式。使用了
golang.org/x/net/route

下面给出一些参考

https://github.com/libp2p/go-netroute

https://github.com/jackpal/gateway/issues/27

https://github.com/GameXG/gonet/blob/master/route/route_windows.go

除了 GetGateway之外，还可以使用更多其他代码
*/
func GetGateway() (ip net.IP, index int, err error) {
	var rib []byte
	rib, err = route.FetchRIB(syscall.AF_INET, syscall.NET_RT_DUMP, 0)
	if err != nil {
		return
	}
	var msgs []route.Message
	msgs, err = route.ParseRIB(syscall.NET_RT_DUMP, rib)
	if err != nil {
		return
	}

	for _, m := range msgs {
		switch m := m.(type) {
		case *route.RouteMessage:
			switch sa := m.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
			case *route.Inet6Addr:
				ip = make(net.IP, net.IPv6len)
				copy(ip, sa.IP[:])
			}
			index = m.Index

			return

		}
	}
	err = errors.New("no gateway")
	return
}

func GetDeviceNameIndex(idx int) string {
	intf, err := net.InterfaceByIndex(idx)
	if err != nil {
		utils.Error(err.Error())
	}
	return intf.Name
}

var rememberedRouterName string

func init() {
	autoRouteFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if len(directList) == 0 {
			utils.Warn(auto_route_bindToDeviceWarn)
		}

		out, err := exec.Command("ifconfig", tunDevName, tunIP, tunGateway, "up").Output()
		if ce := utils.CanLogInfo("auto route setup tun ip"); ce != nil {
			ce.Write(zap.String("output", string(out)))
		}
		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		// out, err = exec.Command("netstat", "-nr", "-f", "inet").Output()

		// if err != nil {
		// 	if ce := utils.CanLogErr("auto route failed"); ce != nil {
		// 		ce.Write(zap.Error(err))
		// 	}
		// 	return
		// }

		// startLineIndex := -1
		// fields := strings.Split(string(out), "\n")
		// for i, l := range fields {
		// 	if strings.HasPrefix(l, "Destination") {
		// 		if i < len(fields)-1 && strings.HasPrefix(fields[i+1], "default") {
		// 			startLineIndex = i + 1

		// 		}
		// 		break
		// 	}
		// }
		// if startLineIndex < 0 {
		// 	utils.Warn("auto route failed, parse netstat output failed,1")
		// 	return
		// }
		// str := utils.StandardizeSpaces(fields[startLineIndex])
		// fields = strings.Split(str, " ")

		// if len(fields) <= 1 {
		// 	utils.Warn("auto route failed, parse netstat output failed,2")
		// 	return

		// }
		// routerIP := fields[1]

		rip, ridx, err := GetGateway() //oops, accidentally rest in peace

		if err != nil {
			if ce := utils.CanLogErr("auto route failed when get gateway"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		rememberedRouterIP = rip.String()
		rname := GetDeviceNameIndex(ridx)
		rememberedRouterName = rname

		if ce := utils.CanLogInfo("auto route: Your router should be"); ce != nil {
			ce.Write(zap.String("ip", rememberedRouterIP), zap.String("name", rname))
		}

		strs := []string{
			"route delete -host default",
			"route add default -interface " + tunDevName + " -hopcount 1",
			"route add -net 0.0.0.0/1 " + rememberedRouterIP + " -hopcount 4",
			"route add " + rememberedRouterIP + " -interface " + rname,
		}

		//这里err只能捕获没有权限运行等错误; 如果路由表修改失败，是不会返回err的

		for _, v := range directList {
			strs = append(strs, "route add -host "+v+" "+rememberedRouterIP)
		}

		if manualRoute {
			promptManual(strs)
		} else {

			if e := utils.LogExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
				return
			}
		}

		utils.Info("auto route succeed!")
	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if rememberedRouterIP == "" {
			return
		}

		strs := []string{
			"route delete -host " + rememberedRouterIP + " -interface " + rememberedRouterName,
			"route delete -host default -interface " + tunDevName,
			"route delete -net 0.0.0.0/1",
			"route add default " + rememberedRouterIP + " -hopcount 1",
		}

		for _, v := range directList {
			strs = append(strs, "route delete -host "+v)
		}

		if manualRoute {
			promptManual(strs)
		} else {

			if e := utils.LogExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}
		}
	}
}
