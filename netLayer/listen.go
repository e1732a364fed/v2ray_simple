package netLayer

import (
	"log"
	"net"
	"os"
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
	case "unix":
		// 参考 https://eli.thegreenplace.net/2019/unix-domain-sockets-in-go/
		//监听 unix domain socket后，就会自动创建 相应文件;
		// 而且程序退出后，该文件不会被删除
		//  而且再次启动后如果遇到了这个文件，就会报错，就像tcp端口已经被监听 的错误一样:
		// “bind: address already in use”
		// 所以必须把原文件删掉
		// 但是问题是，有可能被一些粗心的用户搞出大问题
		// 如果不小心设置成了 '/' 根目录，那我们删的话是不是会直接把所有文件都删掉了？
		// 总之RemoveAll函数千万不能用，Remove函数倒是没什么大事
		if utils.FileExist(addr) {

			if utils.CanLogDebug() {
				log.Println("unix file exist, deleting", addr)
			}
			err := os.Remove(addr)
			if err != nil {
				return utils.NewDataErr("Error when deleting previous unix socket file,", err, addr)
			}

		}
		fallthrough
	default:
		listener, err := net.Listen(network, addr)
		if err != nil {
			return err
		}
		go loopAccept(listener, acceptFunc)

	}
	return nil
}
