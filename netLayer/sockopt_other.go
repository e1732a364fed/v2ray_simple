//go:build !linux
// +build !linux

package netLayer

func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
}
