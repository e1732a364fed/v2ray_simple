//go:build !linux

package tproxy

import "github.com/e1732a364fed/v2ray_simple/utils"

//placeholder for non-linux systems, return utils.ErrNotImplemented
func SetIPTablesByPort(port int) error {
	return utils.ErrUnImplemented
}

//placeholder for non-linux systems
func CleanupIPTables() {
}
