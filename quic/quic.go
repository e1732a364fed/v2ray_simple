//Package quic defines functions to listen and dial quic, with some customizable congestion settings.
package quic

import (
	"context"
	"crypto/tls"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
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

//100mbps
const Default_hysteriaMaxByteCount = 1024 * 1024 / 8 * 100

func CloseSession(baseC any) {
	baseC.(quic.Session).CloseWithError(0, "")
}

//给 quic.Stream 添加 方法使其满足 net.Conn.
// quic.Stream 唯独不支持 LocalAddr 和 RemoteAddr 方法.
// 因为它是通过 StreamID 来识别连接. 不过session是有的。
type StreamConn struct {
	quic.Stream
	laddr, raddr        net.Addr
	relatedSessionState *sessionState
	isclosed            bool
}

func (sc StreamConn) LocalAddr() net.Addr {
	return sc.laddr
}
func (sc StreamConn) RemoteAddr() net.Addr {
	return sc.raddr
}

//这里必须要同时调用 CancelRead 和 CancelWrite
// 因为 quic-go这个设计的是双工的，调用Close实际上只是间接调用了 CancelWrite
// 看 quic-go包中的 quic.SendStream 的注释就知道了.
func (sc StreamConn) Close() error {
	if sc.isclosed {
		return nil
	}
	sc.isclosed = true
	sc.CancelRead(quic.StreamErrorCode(quic.ConnectionRefused))
	sc.CancelWrite(quic.StreamErrorCode(quic.ConnectionRefused))
	if rss := sc.relatedSessionState; rss != nil {

		atomic.AddInt32(&rss.openedStreamCount, -1)

	}
	return sc.Stream.Close()
}

const (
	common_maxidletimeout             = time.Second * 45
	common_HandshakeIdleTimeout       = time.Second * 8
	common_ConnectionIDLength         = 12
	server_maxStreamCountInOneSession = 4
)

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

	}(newConnChan)

	return
}

func isActive(s quic.Session) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

type Client struct {
	knownServerMaxStreamCount int32

	serverAddrStr string

	tlsConf                      tls.Config
	useHysteria, hysteria_manual bool
	maxbyteCount                 int

	clientconns     map[[16]byte]*sessionState
	sessionMapMutex sync.RWMutex
}

type sessionState struct {
	quic.Session
	id [16]byte

	openedStreamCount int32
}

func NewClient(addr *netLayer.Addr, alpnList []string, host string, insecure bool, useHysteria bool, maxbyteCount int, hysteria_manual bool) *Client {
	return &Client{
		serverAddrStr: addr.String(),
		tlsConf: tls.Config{
			InsecureSkipVerify: insecure,
			ServerName:         host,
			NextProtos:         alpnList,
		},
		useHysteria:     useHysteria,
		hysteria_manual: hysteria_manual,
		maxbyteCount:    maxbyteCount,
	}
}

//trimSessions移除不Active的session, 并试图返回一个 最佳的可用于新stream的session
func (c *Client) trimSessions(ss map[[16]byte]*sessionState) (s *sessionState) {
	minSessionNum := 10000
	for id, thisState := range ss {
		if isActive(thisState) {

			if c.knownServerMaxStreamCount == 0 {
				s = thisState
				return
			} else {
				osc := int(thisState.openedStreamCount)

				if osc < int(c.knownServerMaxStreamCount) {

					if osc < minSessionNum {
						s = thisState
						minSessionNum = osc

					}
				}
			}

		} else {
			thisState.CloseWithError(0, "")
			delete(ss, id)
		}
	}

	return
}

func (c *Client) DialCommonConn(openBecausePreviousFull bool, previous any) any {
	//我们采用预先openStream的策略, 来试出哪些session已经满了, 哪些没满
	// 已知的是, 一个session满了之后, 要等待 0～45秒 或以上的时间, 才能它才可能腾出空位

	//我们对每一个session所打开过的stream进行计数，这样就可以探知 服务端 的 最大stream数设置.

	if !openBecausePreviousFull {

		c.sessionMapMutex.Lock()
		var theSession *sessionState
		if len(c.clientconns) > 0 {
			theSession = c.trimSessions(c.clientconns)
		}
		if len(c.clientconns) > 0 {
			c.sessionMapMutex.Unlock()
			if theSession != nil {
				return theSession

			}
		} else {
			c.clientconns = make(map[[16]byte]*sessionState)
			c.sessionMapMutex.Unlock()
		}
	} else if previous != nil && c.knownServerMaxStreamCount == 0 {

		ps, ok := previous.(*sessionState)
		if !ok {
			if ce := utils.CanLogDebug("QUIC: 'previous' parameter was given but with wrong type  "); ce != nil {
				ce.Write(zap.String("type", reflect.TypeOf(previous).String()))
			}
			return nil
		}

		c.knownServerMaxStreamCount = ps.openedStreamCount

		if ce := utils.CanLogDebug("QUIC: knownServerMaxStreamCount"); ce != nil {
			ce.Write(zap.Int32("count", c.knownServerMaxStreamCount))
		}

	}

	session, err := quic.DialAddr(c.serverAddrStr, &c.tlsConf, &common_DialConfig)
	if err != nil {
		if ce := utils.CanLogErr("QUIC:  dial failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil
	}

	if c.useHysteria {
		if c.maxbyteCount <= 0 {
			c.maxbyteCount = Default_hysteriaMaxByteCount
		}

		if c.hysteria_manual {
			bs := NewBrutalSender_M(congestion.ByteCount(c.maxbyteCount))
			session.SetCongestionControl(bs)

		} else {
			bs := NewBrutalSender(congestion.ByteCount(c.maxbyteCount))
			session.SetCongestionControl(bs)

		}
	}

	id := utils.GenerateUUID()

	var result = &sessionState{Session: session, id: id}
	c.sessionMapMutex.Lock()
	c.clientconns[id] = result
	c.sessionMapMutex.Unlock()

	return result
}

func (c *Client) DialSubConn(thing any) (net.Conn, error) {
	theState, ok := thing.(*sessionState)
	if !ok {
		return nil, utils.ErrNilOrWrongParameter
	}
	stream, err := theState.OpenStream()
	if err != nil {

		return nil, err

	}

	atomic.AddInt32(&theState.openedStreamCount, 1)

	return StreamConn{Stream: stream, laddr: theState.LocalAddr(), raddr: theState.RemoteAddr(), relatedSessionState: theState}, nil
}
