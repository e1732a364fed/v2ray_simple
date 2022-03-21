package netLayer

import (
	"log"
	"net"
	"strings"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

func loopAccept(listener net.Listener, acceptFunc func(net.Conn)) {
	for {
		newc, err := listener.Accept()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "closed") {
				if utils.CanLogDebug() {
					log.Println("local connection closed", err)

				}
				break
			}
			if utils.CanLogWarn() {
				log.Println("failed to accept connection: ", err)
			}
			if strings.Contains(errStr, "too many") {
				if utils.CanLogWarn() {
					log.Println("To many incoming conn! Sleep ", errStr)

				}
				time.Sleep(time.Millisecond * 500)
			}
			continue
		}
		go acceptFunc(newc)
	}
}

// ListenAndAccept 试图监听 所有类型的网络，包括tcp, udp 和 unix domain socket.
// 非阻塞，在自己的goroutine中监听.
func ListenAndAccept(network, addr string, acceptFunc func(net.Conn)) error {
	switch network {
	default:
		listener, err := net.Listen(network, addr)
		if err != nil {
			return err
		}
		go loopAccept(listener, acceptFunc)
	case "udp":
		ua, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}
		listener, err := NewUDPListener(ua)
		if err != nil {
			return err
		}
		go loopAccept(listener, acceptFunc)

	}
	return nil
}
