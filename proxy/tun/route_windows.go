package tun

import (
	"fmt"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	/*
		经过测试发现，完全一样的路由命令，自动执行和 手动在控制台输入执行，效果竟然不一样; 手动的能正常运行, 自动的就不行, 怪
		  后发现，是需要等待4秒钟；3秒都不够；

		要确保wintun的 Gateway显示为 On-link, Interface显示为 设置好的地址；
		错误时显示的是 Geteway 是 设置好的地址，Interface为原始路由器的地址

			netsh interface ip set address name="vs_wintun" source=static addr=192.168.123.1 mask=255.255.255.0 gateway=none

			route add vps_ip router_ip
			route add 0.0.0.0 mask 0.0.0.0 vps_ip metric 5

		而且wintun的自动执行行为 和 go-tun2socks 的 tap的行为还是不一样。

		在wintun，如果自动删除原默认路由(0.0.0.0 -> router)，再自动添加新默认路由(0.0.0.0 -> tun)，是添加不上的

		wintun 和 默认路由 都存在时, wintun会优先
	*/
	autoRouteFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {

		if len(directList) == 0 {
			utils.Warn(auto_route_bindToDeviceWarn)
		}

		routerIP, err := netLayer.GetGateway()
		if err != nil {
			return
		}

		if ce := utils.CanLogInfo("auto route: Your router's ip should be"); ce != nil {
			ce.Write(zap.String("ip", routerIP))
		}
		rememberedRouterIP = routerIP

		var strs = []string{
			fmt.Sprintf(`netsh interface ip set address name="%s" source=static addr=%s mask=255.255.255.0 gateway=none`, tunDevName, tunGateway),

			fmt.Sprintf(`netsh interface ip set dns name="%s" static 8.8.8.8`, tunDevName), //windows上，wintun的dns要单独配置一下，如果为空的话，就会走默认路由器，那样就会遭到dns污染

			//https://tweaks.com/windows/40339/configure-ip-address-and-dns-from-command-line/

			//ipconfig /all 查看每个网卡的dns

			//对比来说，darwin上只需要直接设置wifi网卡的dns即可，而windows上因为wintun相当于单独的虚拟网卡，所以就要设这里
		}

		for _, v := range directList {
			strs = append(strs, fmt.Sprintf("route add %s %s metric 5", v, rememberedRouterIP))

		}

		strs = append(strs, fmt.Sprintf("route add 0.0.0.0 mask 0.0.0.0 %s metric 6", rememberedRouterIP))

		strs = append(strs, fmt.Sprintf("route add 0.0.0.0 mask 0.0.0.0 %s metric 6", tunGateway))

		if manualRoute {
			promptManual(strs)
		} else {
			if e := utils.ExecCmdList(strs[:len(strs)-1]); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}

			time.Sleep(time.Second * 4)
			if e := utils.ExecCmd(strs[len(strs)-1]); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}
		}

	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		if rememberedRouterIP == "" {
			return
		}
		//恢复路由表

		strs := []string{
			"route delete 0.0.0.0 mask 0.0.0.0 " + tunGateway,
			"route delete 0.0.0.0 mask 0.0.0.0 " + rememberedRouterIP,
			"route add 0.0.0.0 mask 0.0.0.0 " + rememberedRouterIP + " metric 50",
		}

		for _, v := range directList {
			strs = append(strs, "route delete "+v+" "+rememberedRouterIP)
		}

		if manualRoute {
			promptManual(strs)
		} else {

			if e := utils.ExecCmdList(strs); e != nil {
				if ce := utils.CanLogErr("recover auto route failed"); ce != nil {
					ce.Write(zap.Error(e))
				}
			}
		}

	}
}
