package netLayer

import (
	"net"
	"syscall"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

//经过测试，网速越快、延迟越小，越不需要readv, 此时首包buf越大越好, 因为一次系统底层读取就会读到一大块数据, 此时再用readv分散写入 实际上就是反效果; readv的数量则不需要太多

//在内网单机自己连自己测速时,readv会导致降速.

const (
	DefaultReadvOption = true
)

var (

	// 是否会在转发过程中使用readv
	UseReadv bool
)

// if r!=0, then it means c can be used in readv. -1 means syscall.RawConn,1 means utils.BuffersReader, 2 means  utils.Readver
func IsConnGoodForReadv(c net.Conn) (r int, rawReadConn syscall.RawConn) {
	rawReadConn = GetRawConn(c)

	if rawReadConn != nil {
		r = -1
		return

	} else if mr, ok := c.(utils.MultiReader); ok {
		r = mr.WillReadBuffersBenifit()
		return

	}
	return
}
