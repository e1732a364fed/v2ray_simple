//go:build !(linux || darwin || windows)
// +build !linux,!darwin,!windows

package netLayer

// SetSockOpt 是平台相关的.
func SetSockOpt(fd int, sockopt *Sockopt, isudp bool, isipv6 bool) {
}
