package tun

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	autoRouteFunc = func(tunDevName, tunGateway, tunIP, dns string, directList []string) {
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

		rip, ridx, err := netLayer.GetGateway() //oops, accidentally rest in peace

		if err != nil {
			if ce := utils.CanLogErr("auto route failed when get gateway"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		rememberedRouterIP = rip.String()
		rname := netLayer.GetDeviceNameIndex(ridx)
		rememberedRouterName = rname

		if ce := utils.CanLogInfo("auto route: Your router should be"); ce != nil {
			ce.Write(zap.String("ip", rememberedRouterIP), zap.String("name", rname))
		}

		strs := getRouteStr(true, tunGateway)
		//虽然添加了两个默认路由，但是tun的 hopcount (metric) 更低，所以tun的优先

		for _, v := range directList {
			strs = append(strs, "route add -host "+v+" "+rememberedRouterIP)
		}

		if manualRoute {
			promptManual(strs)
		} else {

			if e := utils.LogExecCmdList(strs); e != nil {
				//这里只能捕获没有权限运行等错误; 如果路由表修改失败，是不会返回err的

				if ce := utils.CanLogErr("auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
				return
			}
		}

		if dns != "" {
			rdnss := netLayer.GetSystemDNS()
			if len(rdnss) > 0 {
				rememberedRouterDns = rdnss[0]
				netLayer.SetSystemDNS(dns)
			}
		}

		utils.Info("auto route succeed!")
	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if rememberedRouterIP == "" {
			return
		}

		strs := getRouteStr(false, tunGateway)

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

		if rememberedRouterDns != "" {
			netLayer.SetSystemDNS(rememberedRouterDns)

		}
	}
}

func firstPart(ip string) string {
	strs := strings.SplitN(ip, ".", 2)
	return strs[0]
}

func getRouteStr(add bool, tunGateway string) []string {

	//https://github.com/xjasonlyu/tun2socks/wiki/Examples

	// 	"route add -net 1.0.0.0/8 " + tunGateway + " -hopcount 1",
	// "route add -net 2.0.0.0/7 " + tunGateway + " -hopcount 1",
	// "route add -net 4.0.0.0/6 " + tunGateway + " -hopcount 1",
	// "route add -net 8.0.0.0/5 " + tunGateway + " -hopcount 1",
	// "route add -net 16.0.0.0/4 " + tunGateway + " -hopcount 1",
	// "route add -net 32.0.0.0/3 " + tunGateway + " -hopcount 1",
	// "route add -net 64.0.0.0/2 " + tunGateway + " -hopcount 1",
	// "route add -net 128.0.0.0/1 " + tunGateway + " -hopcount 1",
	// "route add -net " + firstPart(tunGateway) + ".0.0.0/15 " + tunGateway + " -hopcount 1",

	strs := []string{}

	cmdStr := "delete"
	if add {
		cmdStr = "add"
	}

	for i := 0; i < 8; i++ {
		num := 1 << i
		str := fmt.Sprintf("route %s -net %d.0.0.0/%d %s -hopcount 1", cmdStr, num, 8-i, tunGateway)
		strs = append(strs, str)
	}
	str := fmt.Sprintf("route %s -net %s.0.0.0/15 %s -hopcount 1", cmdStr, firstPart(tunGateway), tunGateway)
	strs = append(strs, str)

	return strs
}
