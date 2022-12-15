package fdbased

import (
	"errors"

	"github.com/e1732a364fed/v2ray_simple/netLayer/tun/device"
)

func Open(name string, mtu uint32) (device.Device, error) {
	return nil, errors.New("not supported")
}
