package tun

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	//https://github.com/xjasonlyu/tun2socks/wiki/Examples

	//通过下面命令可以看到 建立的tun设备
	//ip addr show

	//通过下面命令查看路由
	//ip route show

	autoRoutePreFunc = func(tunDevName, tunGateway, tunIP string, directList []string) bool {
		if len(directList) == 0 {
			utils.Warn(auto_route_bindToDeviceWarn)
		}

		utils.Info("tun auto setup device for linux...")

		e := utils.ExecCmd("ip tuntap add mode tun dev " + tunDevName)
		if e != nil {
			return false
		}

		//todo: 还是要使用mask变量，以便定制这里的 "15"
		e = utils.ExecCmd("ip addr add " + tunGateway + "/15 dev " + tunDevName)
		if e != nil {
			return false
		}

		e = utils.ExecCmd("ip link set dev " + tunDevName + " up")
		if e != nil {
			return false
		}

		return true
	}

	//暂未实现linux 上配置dns的功能
	autoRouteFunc = func(tunDevName, tunGateway, tunIP, dns string, directList []string) {
		routerip, routerName, err := netLayer.GetGateway()
		if err != nil {
			utils.Error(err.Error())
		}
		if routerip == nil || len(routerip) == 0 {
			utils.Error("got router ip but is nil / emtpy")
			return
		}
		rememberedRouterName = routerName

		rememberedRouterIP = routerip.String()

		var strs = []string{
			"ip route del default",
			"ip route add default via " + tunGateway + " dev " + tunDevName + " metric 1",
			"ip route add default via " + rememberedRouterIP + " dev " + routerName + " metric 10",
		}

		//上面命令指定了不同 设备要走不同的网关，并且后面要用到 bindToDevice 功能来让 dial 走 原网卡
		// 这个 bindToDevice 要用户自己配置

		//有了 bindToDevice, linux 就不怕造成回环

		for _, v := range directList {
			strs = append(strs, "ip route add "+v+" via "+rememberedRouterIP+" dev "+routerName+" metric 10")
		}

		if manualRoute {
			promptManual(strs)
		} else {
			if e := utils.ExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("run auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}
		}

	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if rememberedRouterIP == "" || rememberedRouterName == "" {
			return
		}

		var strs = []string{
			//经测试，tun设备被关闭后，相关路由应该自动就恢复了
			//"ip route del default",
			//"ip route add default via " + rememberedRouterIP,

			"ip link set dev " + tunDevName + " down",
		}

		for _, v := range directList {
			strs = append(strs, "ip route del "+v+" via "+rememberedRouterIP+" dev "+rememberedRouterName+" metric 10")
		}

		if manualRoute {
			promptManual(strs)
		} else {

			if e := utils.LogExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
				return

			}

		}
	}

	autoRouteDownAfterCloseFunc = func(tunDevName, tunGateway, tunIP string, directlist []string) {
		if _, e := utils.LogRunCmd("ip", "tuntap", "del", "mode", "tun", "dev", tunDevName); e != nil {
			if ce := utils.CanLogErr("recover auto route after close failed"); ce != nil {
				ce.Write(zap.Error(e))
			}
			return

		}
	}
}
