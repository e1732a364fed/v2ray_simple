package quic

import (
	"crypto/tls"
	"net"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

//implements advLayer.MuxClient
type Client struct {
	knownServerMaxStreamCount int32

	serverAddrStr string

	tlsConf                             tls.Config
	useHysteria, hysteria_manual, early bool
	maxbyteCount                        int

	clientconns  map[[16]byte]*connState
	connMapMutex sync.RWMutex
}

func NewClient(addr *netLayer.Addr, alpnList []string, host string, insecure bool, useHysteria bool, maxbyteCount int, hysteria_manual, early bool) *Client {
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
		early:           early,
	}
}

//trimBadConns removes non-Active sessions, 并试图返回一个 最佳的可用于新stream的session
func (c *Client) trimBadConns() (bestConn *connState) {
	minSessionNum := 10000
	for id, thisState := range c.clientconns {
		if isActive(thisState) {

			if c.knownServerMaxStreamCount == 0 {
				bestConn = thisState
				return
			} else {
				osc := int(thisState.openedStreamCount)

				if osc < int(c.knownServerMaxStreamCount) {

					if osc < minSessionNum {
						bestConn = thisState
						minSessionNum = osc

					}
				}
			}

		} else {
			thisState.CloseWithError(0, "")
			delete(c.clientconns, id)
		}
	}

	return
}

func (c *Client) ProcessWhenFull(previous any) {
	if previous != nil && c.knownServerMaxStreamCount == 0 {

		ps, ok := previous.(*connState)
		if !ok {
			if ce := utils.CanLogDebug("QUIC: 'previous' parameter was given but with wrong type  "); ce != nil {
				ce.Write(zap.String("type", reflect.TypeOf(previous).String()))
			}
			return
		}

		c.knownServerMaxStreamCount = ps.openedStreamCount

		if ce := utils.CanLogDebug("QUIC: knownServerMaxStreamCount"); ce != nil {
			ce.Write(zap.Int32("count", c.knownServerMaxStreamCount))
		}

	}
}

//获取已拨号的连接，或者重新从底层拨号。返回一个可作 c.DialSubConn 参数 的值.
func (c *Client) GetCommonConn(_ net.Conn) (any, error) {
	//返回一个 *sessionState.

	//我们采用预先openStream的策略, 来试出哪些session已经满了, 哪些没满
	// 已知的是, 一个session满了之后, 要等待 0～45秒 或以上的时间, 才能它才可能腾出空位

	//我们对每一个session所打开过的stream进行计数，这样就可以探知 服务端 的 最大stream数设置.

	{

		c.connMapMutex.Lock()
		var theState *connState
		if len(c.clientconns) > 0 {
			theState = c.trimBadConns()
		}
		if len(c.clientconns) > 0 {
			c.connMapMutex.Unlock()
			if theState != nil {
				return theState, nil

			}
		} else {
			c.clientconns = make(map[[16]byte]*connState)
			c.connMapMutex.Unlock()
		}
	}

	var conn quic.Connection
	var err error

	if c.early {
		utils.Info("quic Dial Early")
		conn, err = quic.DialAddrEarly(c.serverAddrStr, &c.tlsConf, &common_DialConfig)

	} else {
		conn, err = quic.DialAddr(c.serverAddrStr, &c.tlsConf, &common_DialConfig)

	}

	if err != nil {
		if ce := utils.CanLogErr("QUIC:  dial failed"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return nil, err
	}

	if c.useHysteria {
		if c.maxbyteCount <= 0 {
			c.maxbyteCount = Default_hysteriaMaxByteCount
		}

		if c.hysteria_manual {
			bs := NewBrutalSender_M(congestion.ByteCount(c.maxbyteCount))
			conn.SetCongestionControl(bs)

		} else {
			bs := NewBrutalSender(congestion.ByteCount(c.maxbyteCount))
			conn.SetCongestionControl(bs)

		}
	}

	id := utils.GenerateUUID()

	var result = &connState{Connection: conn, id: id}
	c.connMapMutex.Lock()
	c.clientconns[id] = result
	c.connMapMutex.Unlock()

	return result, nil
}

func (c *Client) DialSubConn(thing any) (net.Conn, error) {
	theState, ok := thing.(*connState)
	if !ok {
		return nil, utils.ErrNilOrWrongParameter
	}
	stream, err := theState.OpenStream()
	if err != nil {

		return nil, err

	}

	atomic.AddInt32(&theState.openedStreamCount, 1)

	return StreamConn{Stream: stream, laddr: theState.LocalAddr(), raddr: theState.RemoteAddr(), relatedConnState: theState}, nil
}

func (c *Client) IsSuper() bool {
	return true
}

func (c *Client) IsMux() bool {
	return true
}

func (c *Client) IsEarly() bool {
	return c.early
}
func (c *Client) GetPath() string {
	return ""
}
