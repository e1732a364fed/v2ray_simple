package netLayer

import (
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

func GetGateway() (ip string, err error) {
	var out []byte
	out, err = exec.Command("netstat", "-nr").Output()

	if err != nil {
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
		err = utils.ErrFailed
		return
	}
	str := utils.StandardizeSpaces(lines[startLineIndex])
	fields := strings.Split(str, " ")

	if len(fields) <= 3 {
		utils.Warn("auto route failed, parse netstat output failed,2")
		err = utils.ErrFailed

		return
	}

	ip = fields[2]

	if ip == "On-link" {
		utils.Warn("auto route failed, routerIP parse failed, got " + ip)
		err = utils.ErrFailed

		return
	}

	return
}
