//Package quic defines functions to listen and dial quic, with some customizable congestion settings.
package quic

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

//quic的包装太简单了

//超简单，直接参考 https://github.com/lucas-clemente/quic-go/blob/master/example/echo/echo.go

//我们这里利用了hysteria的阻控，但是没有使用hysteria的通知速率和 auth的 数据头，也就是说我们这里是纯quic协议的情况下使用了hysteria的优点。

//但是我在mac里实测，内网单机极速测速的情况下，本来tcp能达到3000mbps的速度，到了quic就只能达到 1333mbps左右。

//我们要是以后不使用hysteria的话，只需删掉 useHysteria 里的代码, 并删掉 go.mod中的replace部分
// 然后proxy.go里的 相关配置部分也要删掉 在 prepareTLS_for* 函数中 的相关配置 即可.

//3000mbps
const default_hysteriaMaxByteCount = 1024 * 1024 / 8 * 3000

func CloseBaseConn(baseC any, t string) {
	switch t {
	case "quic":

		baseC.(quic.Session).CloseWithError(1, "I want to close")
	}
}

//给 quic.Stream 添加 方法使其满足 net.Conn.
// quic.Stream 唯独不支持 LocalAddr 和 RemoteAddr 方法.
// 因为它是通过 StreamID 来识别连接.
type StreamConn struct {
	quic.Stream
}

func (qsc StreamConn) LocalAddr() net.Addr {
	return nil
}
func (qsc StreamConn) RemoteAddr() net.Addr {
	return nil
}

//这里必须要同时调用 CancelRead 和 CancelWrite
// 因为 quic-go这个设计的是双工的，调用Close实际上只是间接调用了 CancelWrite
// 看 quic-go包中的 quic.SendStream 的注释就知道了.
func (qsc StreamConn) Close() error {
	qsc.CancelRead(quic.StreamErrorCode(quic.ConnectionRefused))
	qsc.CancelWrite(quic.StreamErrorCode(quic.ConnectionRefused))
	return qsc.Stream.Close()
}

const (
	our_maxidletimeout       = time.Hour * 2 //time.Second * 45	//idletimeout 设的时间越长越不容易断连.
	our_HandshakeIdleTimeout = time.Second * 8
	our_ConnectionIDLength   = 12
)

var (
	AlpnList = []string{"h3"}

	our_ListenConfig = quic.Config{
		ConnectionIDLength:    our_ConnectionIDLength,
		HandshakeIdleTimeout:  our_HandshakeIdleTimeout,
		MaxIdleTimeout:        our_maxidletimeout,
		MaxIncomingStreams:    32,
		MaxIncomingUniStreams: -1,
		KeepAlive:             true,
	}

	our_DialConfig = quic.Config{
		ConnectionIDLength:   our_ConnectionIDLength,
		HandshakeIdleTimeout: our_HandshakeIdleTimeout,
		MaxIdleTimeout:       our_maxidletimeout,
		KeepAlive:            true,
	}
)

func ListenInitialLayers(addr string, tlsConf tls.Config, useHysteria bool, hysteriaMaxByteCount int, hysteria_manual bool) (newConnChan chan net.Conn, baseConn any) {

	listener, err := quic.ListenAddr(addr, &tlsConf, &our_ListenConfig)
	if err != nil {
		if ce := utils.CanLogErr("quic listen"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if useHysteria {
		if hysteriaMaxByteCount <= 0 {
			hysteriaMaxByteCount = default_hysteriaMaxByteCount
		}

	}

	newConnChan = make(chan net.Conn, 10)

	go func(theChan chan net.Conn) {

		for {
			session, err := listener.Accept(context.Background())
			if err != nil {
				if ce := utils.CanLogErr("quic session accept"); ce != nil {
					ce.Write(zap.Error(err))
				}
				//close(theChan)	//不应关闭chan，因为listen虽然不好使但是也许现存的stream还是好使的...
				return
			}

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
							//只要某个连接idle时间一长，服务端就会出现此错误:
							// timeout: no recent network activity，即 IdleTimeoutError
							//这不能说是错误, 而是quic的udp特性所致，所以放到debug 输出中.

							//我们为了性能，不必将该err转成 net.Error然后判断是否是timeout
							//如果要排错那就开启debug日志即可.

							ce.Write(zap.Error(err))
						}
						break
					}
					theChan <- StreamConn{stream}
				}
			}()
		}

	}(newConnChan)

	return
}

var (
	clientconnMap   = make(map[netLayer.HashableAddr]quic.Session)
	clientconnMutex sync.RWMutex
)

func isActive(s quic.Session) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func DialCommonInitialLayer(serverAddr *netLayer.Addr, tlsConf tls.Config, useHysteria bool, hysteriaMaxByteCount int, hysteria_manual bool) any {

	hash := serverAddr.GetHashable()

	clientconnMutex.RLock()
	existSession, has := clientconnMap[hash]
	clientconnMutex.RUnlock()
	if has {
		if isActive(existSession) {
			return existSession
		}
	}

	session, err := quic.DialAddr(serverAddr.String(), &tlsConf, &our_DialConfig)
	if err != nil {
		if ce := utils.CanLogErr("quic dial"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

	if useHysteria {
		if hysteriaMaxByteCount <= 0 {
			hysteriaMaxByteCount = default_hysteriaMaxByteCount
		}

		if hysteria_manual {
			bs := NewBrutalSender_M(congestion.ByteCount(hysteriaMaxByteCount))
			session.SetCongestionControl(bs)

		} else {
			bs := NewBrutalSender(congestion.ByteCount(hysteriaMaxByteCount))
			session.SetCongestionControl(bs)

		}
	}

	clientconnMutex.Lock()
	clientconnMap[hash] = session
	clientconnMutex.Unlock()

	return session
}

func DialSubConn(thing any) (net.Conn, error) {
	session := thing.(quic.Session)
	stream, err := session.OpenStreamSync(context.Background())
	if err != nil {
		return nil, err
	}
	return StreamConn{stream}, nil
}
