package netLayer

import (
	"errors"
	"net"
	"strings"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/net/route"
)

func init() {
	SetSystemDNS = setSystemDNS
	ToggleSystemProxy = toggleSystemProxy
	GetSystemProxyState = getSystemProxyState
}

/*
我们的auto route使用纯命令行方式。

sing-box 使用了另一种系统级别的方式。使用了
golang.org/x/net/route

下面给出一些参考

https://github.com/libp2p/go-netroute

https://github.com/jackpal/gateway/issues/27

https://github.com/GameXG/gonet/blob/master/route/route_windows.go

除了 GetGateway之外，还可以使用更多其他代码
*/
func GetGateway() (ip net.IP, index int, err error) {
	var rib []byte
	rib, err = route.FetchRIB(syscall.AF_INET, syscall.NET_RT_DUMP, 0)
	if err != nil {
		return
	}
	var msgs []route.Message
	msgs, err = route.ParseRIB(syscall.NET_RT_DUMP, rib)
	if err != nil {
		return
	}

	for _, m := range msgs {
		switch m := m.(type) {
		case *route.RouteMessage:
			switch sa := m.Addrs[syscall.RTAX_GATEWAY].(type) {
			case *route.Inet4Addr:
				ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
			case *route.Inet6Addr:
				ip = make(net.IP, net.IPv6len)
				copy(ip, sa.IP[:])
			}
			index = m.Index

			return

		}
	}
	err = errors.New("no gateway")
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

func GetHardwarePortByInterfaceName(str string) string {
	searchStr := "Device: " + str

	//List All Network Hardware on a Mac via Command Line
	out, err := utils.LogRunCmd("networksetup", "-listallhardwareports")
	if err == nil {
		lines := strings.Split(out, "\n")
		for i, l := range lines {
			if l == searchStr {
				l = lines[i-1]
				return strings.TrimPrefix(l, "Hardware Port: ")
			}
		}
	}
	return ""
}

// helper func, call GetGateway and GetDeviceNameIndex
func GetGatewayDeviceName() (string, error) {
	_, idx, err := GetGateway()
	if err != nil {
		return "", err
	}
	return GetDeviceNameIndex(idx), nil
}

// helper func, call GetGatewayDeviceName and utils.GetDarwinNetAdapterNameByInterfaceName.
//
// hardwarePort 和 interface的名称不同，hardwareport是 Wi-Fi, 而interface.Name 是 en0
func GetDefaultHardwarePort() (string, error) {
	n, e := GetGatewayDeviceName()
	if e != nil {
		return "", utils.ErrInErr{ErrDetail: e, ErrDesc: "GetGatewayDeviceName failed"}
	}
	return GetHardwarePortByInterfaceName(n), nil
}

func setSystemDNS(dns string) {
	hardwareportStr, err := GetDefaultHardwarePort()
	if err != nil {
		if ce := utils.CanLogErr("SetSystemDNS: call GetDefaultHardwarePort failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	utils.LogRunCmd("networksetup", "-setdnsservers", hardwareportStr, dns)

}

/*
darwin 获取wifi名称

/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport -I | grep ' SSID:' | awk '{print $2}'

*/

func GetSystemDNS() string {

	hardwareportStr, err := GetDefaultHardwarePort()
	if err != nil {
		if ce := utils.CanLogErr("GetSystemDNS: call GetDefaultHardwarePort failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return ""
	}

	out, e := utils.LogRunCmd("networksetup", "-getdnsservers", hardwareportStr)
	if e != nil {
		return ""
	}
	return out
}

func toggleSystemProxy(isSocks5 bool, addr, port string, enable bool) {
	//我们使用命令行方式。

	//todo: 还可以参考 https://github.com/getlantern/sysproxy ， 这里用了另一种实现，还用到elevate

	hardwareportStr, err := GetDefaultHardwarePort()
	if err != nil {
		if ce := utils.CanLogErr("ToggleSystemProxy: call GetDefaultHardwarePort failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if isSocks5 {
		if enable {
			utils.LogRunCmd("networksetup", "-setsocksfirewallproxy", hardwareportStr, addr, port)

		} else {
			utils.LogRunCmd("networksetup", "-setsocksfirewallproxystate", hardwareportStr, "off")
		}
	} else {
		if enable {
			utils.LogRunCmd("networksetup", "-setwebproxy", hardwareportStr, addr, port)
			utils.LogRunCmd("networksetup", "-setsecurewebproxy", hardwareportStr, addr, port)

		} else {
			utils.LogRunCmd("networksetup", "-setwebproxystate", hardwareportStr, "off")
			utils.LogRunCmd("networksetup", "-setsecurewebproxystate", hardwareportStr, "off")
		}
	}

}

func getSystemProxyState(isSocks5 bool) (ok, enabled bool, addr, port string) {

	hardwareportStr, err := GetDefaultHardwarePort()
	if err != nil {
		if ce := utils.CanLogErr("ToggleSystemProxy: call GetDefaultHardwarePort failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	var out string
	if isSocks5 {
		var e error
		out, e = utils.LogRunCmd("networksetup", "-getsocksfirewallproxy", hardwareportStr)
		if e != nil {
			return
		}
		ok = true

	} else {
		var e error
		out, e = utils.LogRunCmd("networksetup", "-getwebproxy", hardwareportStr)
		if e != nil {
			return
		}
		ok = true
	}
	strs := strings.Split(out, "\n")
	if len(strs) < 1 {
		return
	}

	if strings.Contains(strs[0], "Yes") {
		enabled = true
	}
	if len(strs) < 3 {
		return
	}
	if strings.Contains(strs[1], "Server: ") {
		addr = strings.TrimPrefix(strs[1], "Server: ")
	}
	if strings.Contains(strs[2], "Port: ") {
		port = strings.TrimPrefix(strs[2], "Port: ")
	}

	return
}
