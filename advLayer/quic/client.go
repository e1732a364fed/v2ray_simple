package quic

import (
	"crypto/tls"
	"net"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

type Client struct {
	knownServerMaxStreamCount int32

	serverAddrStr string

	tlsConf                      tls.Config
	useHysteria, hysteria_manual bool
	maxbyteCount                 int

	clientconns     map[[16]byte]*sessionState
	sessionMapMutex sync.RWMutex
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

//获取已拨号的连接，或者重新从底层拨号。返回一个可作 c.DialSubConn 参数 的值.
func (c *Client) DialCommonConn(openBecausePreviousFull bool, previous any) any {
	//返回一个 *sessionState.

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

	var result = &sessionState{Connection: session, id: id}
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
