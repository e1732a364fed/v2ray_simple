package tun

import (
	"errors"
	"log"
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

除了 GetGateway之外，还可以使用更多其他代码
*/
func GetGateway() (ip net.IP, err error) {
	rib, err := route.FetchRIB(syscall.AF_INET, syscall.NET_RT_DUMP, 0)
	if err != nil {
		return nil, err
	}

	msgs, err := route.ParseRIB(syscall.NET_RT_DUMP, rib)
	if err != nil {
		return nil, err
	}

	for _, m := range msgs {
		switch m := m.(type) {
		case *route.RouteMessage:
			var ip net.IP
			switch sa := m.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
				return ip, nil
			case *route.Inet6Addr:
				ip = make(net.IP, net.IPv6len)
				copy(ip, sa.IP[:])
				return ip, nil
			}
		}
	}
	return nil, errors.New("no gateway")
}

func init() {
	autoRouteFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if len(directList) == 0 {
			utils.Warn("tun auto route called, but no direct list given. auto route will not run.")
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

		rip, err := GetGateway()

		if err != nil {
			if ce := utils.CanLogErr("auto route failed when get gateway"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		rememberedRouterIP = rip.String()

		if ce := utils.CanLogInfo("auto route: Your router's ip should be"); ce != nil {
			ce.Write(zap.String("ip", rememberedRouterIP))
		}

		out1, err := exec.Command("route", "delete", "-host", "default").Output()

		//这里err只能捕获没有权限运行等错误; 如果路由表修改失败，是不会返回err的

	checkErrStep:
		if ce := utils.CanLogInfo("auto route delete default"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		out1, err = exec.Command("route", "add", "default", "-interface", tunDevName).Output()
		if err != nil {
			goto checkErrStep
		}
		if ce := utils.CanLogInfo("auto route add tun"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		for _, v := range directList {
			out1, err = exec.Command("route", "add", "-host", v, rememberedRouterIP).Output()
			if err != nil {
				goto checkErrStep
			}
			if ce := utils.CanLogInfo("auto route add direct"); ce != nil {
				ce.Write(zap.String("output", string(out1)))
			}
		}

		utils.Info("auto route succeed!")
	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if rememberedRouterIP == "" {
			return
		}
		if len(directList) == 0 {
			utils.Warn("tun auto route down called, but no direct list given. auto route will not run.")
		}

		strs := []string{
			"route delete -host default",
			"route add default " + rememberedRouterIP,
		}

		for _, v := range directList {
			strs = append(strs, "route delete -host "+v)
		}

		if manualRoute {
			promptManual(strs)
		} else {
			log.Println("running these commands", strs)

			if e := utils.ExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}
		}
	}
}
