// Package tun provides TUN which implemented device.Device interface.
package tun

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer/tun/device"
)

const Driver = "tun"

func (t *TUN) Type() string {
	return Driver
}

var _ device.Device = (*TUN)(nil)
