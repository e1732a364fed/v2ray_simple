//Package quic defines functions to listen and dial quic, with some customizable congestion settings.
//
// 这里我们使用 hysteria的 brutal阻控.
// 见 https://github.com/tobyxdd/quic-go 中 toby的 *-mod 分支, 里面会多一个 congestion 文件夹.
package quic

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"reflect"
	"time"

	"github.com/hahahrfool/v2ray_simple/advLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

func init() {
	advLayer.ProtocolsMap["quic"] = true
}

//quic的包装太简单了

//超简单，直接参考 https://github.com/lucas-clemente/quic-go/blob/master/example/echo/echo.go

//我们这里利用了hysteria的阻控，但是没有使用hysteria的通知速率和 auth的 数据头，也就是说我们这里是纯quic协议的情况下使用了hysteria的优点。

//但是我在mac里实测，内网单机极速测速的情况下，本来tcp能达到3000mbps的速度，到了quic就只能达到 1333mbps左右。

//我们要是以后不使用hysteria的话，只需删掉 useHysteria 里的代码, 删掉 pacer.go/brutal.go, 并删掉 go.mod中的replace部分.
// 然后proxy.go里的 相关配置部分也要删掉 在 prepareTLS_for* 函数中 的相关配置 即可.

const (
	//100mbps
	Default_hysteriaMaxByteCount = 1024 * 1024 / 8 * 100

	common_maxidletimeout             = time.Second * 45
	common_HandshakeIdleTimeout       = time.Second * 8
	common_ConnectionIDLength         = 12
	server_maxStreamCountInOneSession = 4 //一个session中 stream越多, 性能越低, 因此我们这里限制为4
)

func isActive(s quic.Connection) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func CloseConn(baseC any) {
	qc, ok := baseC.(quic.Connection)
	if ok {
		qc.CloseWithError(0, "")
	} else {
		log.Panicln("quic.CloseConn called with illegal parameter", reflect.TypeOf(baseC).String(), baseC)
	}
}

var (
	AlpnList = []string{"h3"}

	common_ListenConfig = quic.Config{
		ConnectionIDLength:    common_ConnectionIDLength,
		HandshakeIdleTimeout:  common_HandshakeIdleTimeout,
		MaxIdleTimeout:        common_maxidletimeout,
		MaxIncomingStreams:    server_maxStreamCountInOneSession,
		MaxIncomingUniStreams: -1,
		KeepAlive:             true,
	}

	common_DialConfig = quic.Config{
		ConnectionIDLength:   common_ConnectionIDLength,
		HandshakeIdleTimeout: common_HandshakeIdleTimeout,
		MaxIdleTimeout:       common_maxidletimeout,
		KeepAlive:            true,
	}
)

func ListenInitialLayers(addr string, tlsConf tls.Config, useHysteria bool, hysteriaMaxByteCount int, hysteria_manual bool, customMaxStreamCountInOneSession int64) (newConnChan chan net.Conn, baseConn any) {

	thisConfig := common_ListenConfig
	if customMaxStreamCountInOneSession > 0 {
		thisConfig.MaxIncomingStreams = customMaxStreamCountInOneSession
	}

	listener, err := quic.ListenAddr(addr, &tlsConf, &thisConfig)
	if err != nil {
		if ce := utils.CanLogErr("quic listen"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if useHysteria {
		if hysteriaMaxByteCount <= 0 {
			hysteriaMaxByteCount = Default_hysteriaMaxByteCount
		}

	}

	newConnChan = make(chan net.Conn, 10)

	go loopAccept(listener, newConnChan, useHysteria, hysteria_manual, hysteriaMaxByteCount)

	return
}

//阻塞
func loopAccept(listener quic.Listener, theChan chan net.Conn, useHysteria bool, hysteria_manual bool, hysteriaMaxByteCount int) {
	for {
		session, err := listener.Accept(context.Background())
		if err != nil {
			if ce := utils.CanLogErr("quic session accept"); ce != nil {
				ce.Write(zap.Error(err))
			}
			//close(theChan)	//不应关闭chan，因为listen虽然不好使但是也许现存的stream还是好使的...
			return
		}

		dealNewSession(session, theChan, useHysteria, hysteria_manual, hysteriaMaxByteCount)
	}
}

//非阻塞
func dealNewSession(session quic.Connection, theChan chan net.Conn, useHysteria bool, hysteria_manual bool, hysteriaMaxByteCount int) {

	if useHysteria {

		if hysteria_manual {
			bs := NewBrutalSender_M(congestion.ByteCount(hysteriaMaxByteCount))

			session.SetCongestionControl(bs)
		} else {
			bs := NewBrutalSender(congestion.ByteCount(hysteriaMaxByteCount))

			session.SetCongestionControl(bs)
		}

	}

	go func() {
		for {
			stream, err := session.AcceptStream(context.Background())
			if err != nil {
				if ce := utils.CanLogDebug("quic stream accept failed"); ce != nil {
					//只要某个连接idle时间一长，超过了idleTimeout，服务端就会出现此错误:
					// timeout: no recent network activity，即 IdleTimeoutError
					//这不能说是错误, 而是quic的udp特性所致，所以放到debug 输出中.
					//这也同时说明, keep alive功能并不会更新 idle的最后期限.

					//我们为了性能，不必将该err转成 net.Error然后判断是否是timeout
					//如果要排错那就开启debug日志即可.

					ce.Write(zap.Error(err))
				}
				break
			}
			theChan <- StreamConn{stream, session.LocalAddr(), session.RemoteAddr(), nil, false}
		}
	}()
}
