package tun

import (
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

var rememberedRouterIP string

func init() {
	autoRouteFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		var ok = false
		params := "-nr -f inet"
		out, err := exec.Command("netstat", strings.Split(params, " ")...).Output()
		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}
		//log.Println(string(out))
		lines := strings.Split(string(out), "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, "Destination") {
				if i < len(lines)-1 && strings.HasPrefix(lines[i+1], "default") {
					str := utils.StandardizeSpaces(lines[i+1])
					lines = strings.Split(str, " ")
					//log.Println(lines, len(lines))

					if len(lines) > 1 {
						routerIP := lines[1]
						if ce := utils.CanLogInfo("auto route: Your router's ip should be"); ce != nil {
							ce.Write(zap.String("ip", routerIP))
						}
						rememberedRouterIP = routerIP

						params1 := "delete -host default"
						out1, err := exec.Command("route", strings.Split(params1, " ")...).Output()

						//这里err只能捕获没有权限运行等错误; 如果路由表修改失败，是不会返回err的

					errStep:
						if ce := utils.CanLogInfo("auto route delete default"); ce != nil {
							ce.Write(zap.String("output", string(out1)))
						}

						if err != nil {
							if ce := utils.CanLogErr("auto route failed"); ce != nil {
								ce.Write(zap.Error(err))
							}
							return
						}

						params1 = "add default -interface " + tunDevName
						out1, err = exec.Command("route", strings.Split(params1, " ")...).Output()
						if err != nil {
							goto errStep
						}
						if ce := utils.CanLogInfo("auto route add tun"); ce != nil {
							ce.Write(zap.String("output", string(out1)))
						}

						for _, v := range directList {
							params1 = "add -host " + v + " " + rememberedRouterIP
							out1, err = exec.Command("route", strings.Split(params1, " ")...).Output()
							if err != nil {
								goto errStep
							}
							if ce := utils.CanLogInfo("auto route add direct"); ce != nil {
								ce.Write(zap.String("output", string(out1)))
							}
						}

						ok = true
					}
				}
				break
			}
		}
		if !ok {
			utils.Warn("auto route failed")
		}
	}

	autoRouteDownFunc = func(tunDevName, tunGateway, tunIP string, directList []string) {
		//恢复路由表
		params := "delete -host default"
		out1, _ := exec.Command("route", strings.Split(params, " ")...).Output()

		if ce := utils.CanLogInfo("auto route delete tun route"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		params = "add default " + rememberedRouterIP
		out1, _ = exec.Command("route", strings.Split(params, " ")...).Output()

		if ce := utils.CanLogInfo("auto route recover default route"); ce != nil {
			ce.Write(zap.String("output", string(out1)))
		}

		for _, v := range directList {
			params = "delete -host " + v
			out1, _ = exec.Command("route", strings.Split(params, " ")...).Output()

			if ce := utils.CanLogInfo("auto route delete direct"); ce != nil {
				ce.Write(zap.String("output", string(out1)))
			}
		}
	}
}
