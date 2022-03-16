package netLayer

import (
	"net"

	"github.com/yl2chen/cidranger"
)

type TargetDescription struct {
}

//任意一个参数匹配后，都将发往相同的方向，具体房向并不是 SameGroup 所关心的
// SameGroup只负责把一些属性相同的 “网络层特征” 放到一起
type SameGroup struct {
	NetRanger cidranger.Ranger
	IPMap     map[string]net.IP
	DomainMap map[string]string
	TagMap    map[string]bool
}
