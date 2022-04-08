package netLayer

import (
	"net"
	"testing"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

func TestUDP(t *testing.T) {
	//测试setdeadline的情况. 实测证明 SetReadDeadline 在Read过程中也可以使用， 这样就可以防止阻塞

	laddr, _ := net.ResolveUDPAddr("udp", ":"+RandPortStr())

	udpConn, _ := net.ListenUDP("udp", laddr)

	go func() {
		time.Sleep(time.Second)
		udpConn.SetReadDeadline(time.Now())
	}()
	udpConn.ReadFrom(utils.GetPacket())
	t.Log("ok")
}
