package tun

import (
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var rememberedRouterIP string

func init() {
	//经过测试发现，完全一样的路由命令，自动执行和 手动在控制台输入执行，效果竟然不一样; 手动的能正常运行, 自动的就不行, 怪
	/*
		netsh interface ip set address name="vs_wintun" source=static addr=192.168.123.1 mask=255.255.255.0 gateway=none

		route add vps_ip router_ip
		route add 0.0.0.0 mask 0.0.0.0 vps_ip metric 5
	*/
	autoRouteFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {

		out, err := exec.Command("netstat", "-nr").Output()
		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		lines := strings.Split(string(out), "\n")
		startLineIndex := -1
		for i, l := range lines {
			if strings.HasPrefix(l, "IPv4 Route Table") {
				if i < len(lines)-3 && strings.HasPrefix(lines[i+3], "Network") {
					//应该第一行就是默认的路由
					startLineIndex = i + 4
				}
				break
			}
		}

		if startLineIndex < 0 {
			utils.Warn("auto route failed, parse netstat output failed,1")
			return
		}
		str := utils.StandardizeSpaces(lines[startLineIndex])
		fields := strings.Split(str, " ")

		if len(fields) <= 3 {
			utils.Warn("auto route failed, parse netstat output failed,2")
			return
		}

		routerIP := fields[2]
		//为了简单起见，只认为192开头的是我们的本地路由地址;
		if routerIP == "On-link" || !strings.HasPrefix(routerIP, "192") {
			utils.Warn("auto route failed, routerIP parse failed, got " + routerIP)
			return
		}
		if ce := utils.CanLogInfo("auto route: Your router's ip should be"); ce != nil {
			ce.Write(zap.String("ip", routerIP))
		}
		rememberedRouterIP = routerIP

		// params1 := "delete 0.0.0.0 mask 0.0.0.0"
		// out1, err := exec.Command("route", strings.Split(params1, " ")...).Output()

		//这里err只能捕获没有权限运行等错误; 如果路由表修改失败，是不会返回err的

		// if ce := utils.CanLogInfo("auto route delete default"); ce != nil {
		// 	ce.Write(zap.String("output", string(out1)))
		// }

		_, err = exec.Command("netsh", "interface", "ip", "set", "address", `name="`+tunGateway+`"`, "source=static", "addr="+tunGateway, "mask=255.255.255.0", "gateway=none").Output()
		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		out1, err := exec.Command("route", "add", "0.0.0.0", "mask", "0.0.0.0", tunGateway, "metric", "6").Output()
		if err != nil {
			if err != nil {
				if ce := utils.CanLogErr("auto route failed"); ce != nil {
					ce.Write(zap.Error(err))
				}
				return
			}
		}
		if ce := utils.CanLogInfo("auto route add tun"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		for _, v := range directList {
			out1, err = exec.Command("route", "add", v, rememberedRouterIP, "metric", "5").Output()
			if err != nil {
				if err != nil {
					if ce := utils.CanLogErr("auto route failed"); ce != nil {
						ce.Write(zap.Error(err))
					}
					return
				}
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
		//恢复路由表
		params := "delete 0.0.0.0 mask 0.0.0.0"
		out1, _ := exec.Command("route", strings.Split(params, " ")...).Output()

		if ce := utils.CanLogInfo("auto route delete tun route"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		params = "add 0.0.0.0 mask 0.0.0.0 " + rememberedRouterIP + " metric 50"
		out1, _ = exec.Command("route", strings.Split(params, " ")...).Output()

		if ce := utils.CanLogInfo("auto route recover default route"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		for _, v := range directList {
			params = "delete " + v + " " + rememberedRouterIP
			out1, _ = exec.Command("route", strings.Split(params, " ")...).Output()

			if ce := utils.CanLogInfo("auto route delete direct"); ce != nil {
				ce.Write(zap.String("output", string(out1)))
			}
		}
	}
}
