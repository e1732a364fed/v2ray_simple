package netLayer

import (
	"net"
	"os"
)

//用于 listen和 dial 配置一些底层参数.
type Sockopt struct {
	TProxy bool `toml:"tproxy"`
	Somark int  `toml:"mark"`
}

type ListenerWithFile interface {
	net.Listener
	File() (f *os.File, err error)
}

type ConnWithFile interface {
	net.Conn
	File() (f *os.File, err error)
}
