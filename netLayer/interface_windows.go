package netLayer

import (
	"os/exec"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	ToggleSystemProxy = toggleSystemProxy
}

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

func toggleSystemProxy(isSocks5 bool, addr, port string, enable bool) {
	//我们使用命令行方式。

	//todo: 还可以参考 https://github.com/getlantern/sysproxy ， 这里用了另一种实现，还用到elevate

	const inetSettings = `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	if enable {

		utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f")
		addr = addr + ":" + port

		if isSocks5 {
			utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyServer", "/d", "socks="+addr, "/f")

		} else {

			utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyServer", "/d", "http="+addr+";https="+addr, "/f")

		}

		utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyOverride", "/t", "REG_SZ", "/d", "<-loopback>", "/f")

	} else {
		utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f")

		utils.LogRunCmd("reg", "add", inetSettings, "/v", "ProxyServer", "/d", "", "/f")

		utils.LogRunCmd("reg", "delete", inetSettings, "/v", "ProxyOverride", "/f")
	}

}
