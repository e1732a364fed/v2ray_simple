package netLayer

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

func loopAccept(listener net.Listener, acceptFunc func(net.Conn)) {
	for {
		newc, err := listener.Accept()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "closed") {
				if ce := utils.CanLogDebug("local connection closed"); ce != nil {
					//log.Println("local connection closed", err)
					ce.Write(zap.Error(err))

				}
				break
			}
			if ce := utils.CanLogWarn("failed to accept connection"); ce != nil {
				//log.Println("failed to accept connection: ", err)
				ce.Write(zap.Error(err))
			}
			if strings.Contains(errStr, "too many") {
				if ce := utils.CanLogWarn("To many incoming conn! Will Sleep."); ce != nil {
					//log.Println("To many incoming conn! Sleep ", errStr)
					ce.Write(zap.String("err", errStr))

				}
				time.Sleep(time.Millisecond * 500)
			}
			continue
		}
		go acceptFunc(newc)
	}
}

// ListenAndAccept 试图监听 所有类型的网络，包括tcp, udp 和 unix domain socket.
//
// 非阻塞，在自己的goroutine中监听.
func ListenAndAccept(network, addr string, acceptFunc func(net.Conn)) error {
	switch network {
	case "udp", "udp4", "udp6":
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

			if ce := utils.CanLogDebug("unix file exist"); ce != nil {
				//log.Println("unix file exist, deleting", addr)
				ce.Write(zap.String("deleting", addr))
			}
			err := os.Remove(addr)
			if err != nil {
				return utils.ErrInErr{ErrDesc: "Error when deleting previous unix socket file,", ErrDetail: err, Data: addr}
			}

		}
		fallthrough
	default:
		if network == "" {
			network = "tcp"
		}
		listener, err := net.Listen(network, addr)
		if err != nil {
			return err
		}
		go loopAccept(listener, acceptFunc)

	}
	return nil
}
