package tun

import (
	"os/exec"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

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

		strs := []string{
			"route delete -host default",
			"route add default -interface " + tunDevName + " -hopcount 1",
			//"route add -net 0.0.0.0/1 " + rememberedRouterIP + " -hopcount 4",
			"route add " + rememberedRouterIP + " -interface " + rname,
		}
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
