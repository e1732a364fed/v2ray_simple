package tproxy

import (
	"fmt"
	"os/exec"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

//配置iptables

const toutyRaterIptableCmdList = `ip rule add fwmark 1 table 100
ip route add local 0.0.0.0/0 dev lo table 100
iptables -t mangle -N V2RAY
iptables -t mangle -A V2RAY -d 127.0.0.1/32 -j RETURN
iptables -t mangle -A V2RAY -d 224.0.0.0/4 -j RETURN
iptables -t mangle -A V2RAY -d 255.255.255.255/32 -j RETURN
iptables -t mangle -A V2RAY -d 192.168.0.0/16 -p tcp -j RETURN
iptables -t mangle -A V2RAY -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
iptables -t mangle -A V2RAY -p udp -j TPROXY --on-port %d --tproxy-mark 1
iptables -t mangle -A V2RAY -p tcp -j TPROXY --on-port %d --tproxy-mark 1
iptables -t mangle -A PREROUTING -j V2RAY
iptables -t mangle -N V2RAY_MASK
iptables -t mangle -A V2RAY_MASK -d 224.0.0.0/4 -j RETURN
iptables -t mangle -A V2RAY_MASK -d 255.255.255.255/32 -j RETURN
iptables -t mangle -A V2RAY_MASK -d 192.168.0.0/16 -p tcp -j RETURN
iptables -t mangle -A V2RAY_MASK -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
iptables -t mangle -A V2RAY_MASK -j RETURN -m mark --mark 0xff
iptables -t mangle -A V2RAY_MASK -p udp -j MARK --set-mark 1
iptables -t mangle -A V2RAY_MASK -p tcp -j MARK --set-mark 1
iptables -t mangle -A OUTPUT -j V2RAY_MASK`

const iptableRMCmdList = `ip rule del fwmark 1 table 100
ip route del local 0.0.0.0/0 dev lo table 100
iptables -t mangle -D V2RAY -d 127.0.0.1/32 -j RETURN
iptables -t mangle -D V2RAY -d 224.0.0.0/4 -j RETURN
iptables -t mangle -D V2RAY -d 255.255.255.255/32 -j RETURN
iptables -t mangle -D V2RAY -d 192.168.0.0/16 -p tcp -j RETURN
iptables -t mangle -D V2RAY -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
iptables -t mangle -D V2RAY -p udp -j TPROXY --on-port %d --tproxy-mark 1
iptables -t mangle -D V2RAY -p tcp -j TPROXY --on-port %d --tproxy-mark 1
iptables -t mangle -D PREROUTING -j V2RAY
iptables -t mangle -D V2RAY_MASK -d 224.0.0.0/4 -j RETURN
iptables -t mangle -D V2RAY_MASK -d 255.255.255.255/32 -j RETURN
iptables -t mangle -D V2RAY_MASK -d 192.168.0.0/16 -p tcp -j RETURN
iptables -t mangle -D V2RAY_MASK -d 192.168.0.0/16 -p udp ! --dport 53 -j RETURN
iptables -t mangle -D V2RAY_MASK -j RETURN -m mark --mark 0xff
iptables -t mangle -D V2RAY_MASK -p udp -j MARK --set-mark 1
iptables -t mangle -D V2RAY_MASK -p tcp -j MARK --set-mark 1
iptables -t mangle -D OUTPUT -j V2RAY_MASK
iptables -t mangle -F V2RAY
iptables -t mangle -X V2RAY
iptables -t mangle -F V2RAY_MASK
iptables -t mangle -X V2RAY_MASK`

var lastPortSet int

// commands from https://toutyrater.github.io/app/tproxy.html
func SetRouteByPort(port int) error {

	cmd1 := exec.Command("iptables", "-V")
	if err := cmd1.Run(); err != nil {
		return err
	}
	lastPortSet = port

	return utils.ExecCmdMultilineList(fmt.Sprintf(toutyRaterIptableCmdList, port, port))
}

// port 12345
func SetIPTablesByDefault() error {

	return SetRouteByPort(12345)
}

// port 12345
func CleanupIPTablesByDefault() {
	utils.ExecCmdMultilineList(fmt.Sprintf(iptableRMCmdList, 12345, 12345))
}

// clear iptables set by the last SetRouteByPort call
func CleanupRoutes() {
	if lastPortSet != 0 {
		utils.ExecCmdMultilineList(fmt.Sprintf(iptableRMCmdList, lastPortSet, lastPortSet))
		lastPortSet = 0
	}
}
