package tun

import (
	"log"
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

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

		out, err = exec.Command("netstat", "-nr", "-f", "inet").Output()

		if err != nil {
			if ce := utils.CanLogErr("auto route failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		startLineIndex := -1
		fields := strings.Split(string(out), "\n")
		for i, l := range fields {
			if strings.HasPrefix(l, "Destination") {
				if i < len(fields)-1 && strings.HasPrefix(fields[i+1], "default") {
					startLineIndex = i + 1

				}
				break
			}
		}
		if startLineIndex < 0 {
			utils.Warn("auto route failed, parse netstat output failed,1")
			return
		}
		str := utils.StandardizeSpaces(fields[startLineIndex])
		fields = strings.Split(str, " ")

		if len(fields) <= 1 {
			utils.Warn("auto route failed, parse netstat output failed,2")
			return

		}
		routerIP := fields[1]
		if ce := utils.CanLogInfo("auto route: Your router's ip should be"); ce != nil {
			ce.Write(zap.String("ip", routerIP))
		}
		rememberedRouterIP = routerIP

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
			"route delete delete -host default",
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
