package quic

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

//non-blocking
func ListenInitialLayers(addr string, tlsConf tls.Config, useHysteria bool, hysteriaMaxByteCount int, hysteria_manual, early bool, customMaxStreamCountInOneConn int64) (newConnChan chan net.Conn, baseConn io.Closer) {

	thisConfig := common_ListenConfig
	if customMaxStreamCountInOneConn > 0 {
		thisConfig.MaxIncomingStreams = customMaxStreamCountInOneConn
	}

	var listener quic.Listener
	var elistener quic.EarlyListener
	var err error

	if early {
		utils.Info("quic Listen Early")
		elistener, err = quic.ListenAddrEarly(addr, &tlsConf, &thisConfig)

	} else {
		listener, err = quic.ListenAddr(addr, &tlsConf, &thisConfig)

	}
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

	if early {
		go loopAcceptEarly(elistener, newConnChan, useHysteria, hysteria_manual, hysteriaMaxByteCount)
		baseConn = elistener
	} else {
		go loopAccept(listener, newConnChan, useHysteria, hysteria_manual, hysteriaMaxByteCount)

		baseConn = listener
	}

	return
}

//阻塞
func loopAccept(l quic.Listener, theChan chan net.Conn, useHysteria bool, hysteria_manual bool, hysteriaMaxByteCount int) {
	for {
		conn, err := l.Accept(context.Background())
		if err != nil {
			if ce := utils.CanLogErr("quic accept failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			//close(theChan)	//不应关闭chan，因为listen虽然不好使但是也许现存的stream还是好使的...
			return
		}

		if useHysteria {
			configHyForConn(conn, hysteria_manual, hysteriaMaxByteCount)
		}

		go dealNewConn(conn, theChan)
	}
}

//阻塞
func loopAcceptEarly(el quic.EarlyListener, theChan chan net.Conn, useHysteria bool, hysteria_manual bool, hysteriaMaxByteCount int) {

	for {
		conn, err := el.Accept(context.Background())
		if err != nil {
			if ce := utils.CanLogErr("quic early accept failed"); ce != nil {
				ce.Write(zap.Error(err))
			}
			return
		}

		if useHysteria {
			configHyForConn(conn, hysteria_manual, hysteriaMaxByteCount)
		}

		go dealNewConn(conn, theChan)
	}
}

func configHyForConn(conn quic.Connection, hysteria_manual bool, hysteriaMaxByteCount int) {
	if hysteria_manual {
		bs := NewBrutalSender_M(congestion.ByteCount(hysteriaMaxByteCount))

		conn.SetCongestionControl(bs)
	} else {
		bs := NewBrutalSender(congestion.ByteCount(hysteriaMaxByteCount))

		conn.SetCongestionControl(bs)
	}
}

//阻塞
func dealNewConn(conn quic.Connection, theChan chan net.Conn) {

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			if ce := utils.CanLogDebug("quic stream accept failed"); ce != nil {
				//只要某个连接idle时间一长，超过了idleTimeout，服务端就会出现此错误:
				// timeout: no recent network activity，即 quic.IdleTimeoutError
				//这不能说是错误, 而是quic的udp特性所致，所以放到debug 输出中.
				//这也同时说明, keep alive功能并不会更新 idle的最后期限.

				//我们为了性能，不必将该err转成 net.Error然后判断是否是timeout
				//如果要排错，开启debug日志即可.

				ce.Write(zap.Error(err))
			}
			break
		}
		theChan <- StreamConn{stream, conn.LocalAddr(), conn.RemoteAddr(), nil, false}
	}
}

type Server struct {
	addr                          string
	tlsConf                       tls.Config
	useHysteria                   bool
	hysteriaMaxByteCount          int
	hysteria_manual, early        bool
	customMaxStreamCountInOneConn int64
}

func (s *Server) GetPath() string {
	return ""
}

func (*Server) IsMux() bool {
	return true
}

func (*Server) IsSuper() bool {
	return true
}

func (s *Server) StartListen() (newSubConnChan chan net.Conn, baseConn io.Closer) {
	return ListenInitialLayers(s.addr, s.tlsConf, s.useHysteria, s.hysteriaMaxByteCount, s.hysteria_manual, s.early, s.customMaxStreamCountInOneConn)
}

func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn) {
	go dealNewConn(underlay.(quic.Connection), newSubConnChan)
}
