package quic

import (
	"context"
	"crypto/tls"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/congestion"
	"go.uber.org/zap"
)

//non-blocking
func ListenInitialLayers(addr string, tlsConf tls.Config, arg arguments) (newConnChan chan net.Conn, returnCloser io.Closer) {

	thisConfig := common_ListenConfig
	if arg.customMaxStreamsInOneConn > 0 {
		thisConfig.MaxIncomingStreams = arg.customMaxStreamsInOneConn
	}

	var listener quic.Listener
	var elistener quic.EarlyListener
	var err error

	//自己listen，而不是调用 quic.ListenAddr, 这样可以为以后支持 udp的 proxy protocol v2 作准备。

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		if ce := utils.CanLogErr("Failed in QUIC ResolveUDPAddr"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		if ce := utils.CanLogErr("Failed in QUIC listen udp"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if arg.early {
		utils.Info("quic Listen Early")
		elistener, err = quic.ListenEarly(conn, &tlsConf, &thisConfig)

	} else {

		listener, err = quic.Listen(conn, &tlsConf, &thisConfig)

	}
	if err != nil {
		if ce := utils.CanLogErr("Failed in QUIC listen"); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}

	if arg.useHysteria {
		if arg.hysteriaMaxByteCount <= 0 {
			arg.hysteriaMaxByteCount = Default_hysteriaMaxByteCount
		}
	}

	newConnChan = make(chan net.Conn, 10)

	if arg.early {
		go loopAcceptEarly(elistener, newConnChan, arg.useHysteria, arg.hysteria_manual, arg.hysteriaMaxByteCount)
		returnCloser = elistener
	} else {
		go loopAccept(listener, newConnChan, arg.useHysteria, arg.hysteria_manual, arg.hysteriaMaxByteCount)

		returnCloser = listener
	}

	return
}

//阻塞
func loopAccept(l quic.Listener, theChan chan net.Conn, useHysteria bool, hysteria_manual bool, hysteriaMaxByteCount int) {
	for {

		conn, err := l.Accept(context.Background())
		if err != nil {
			if ce := utils.CanLogErr("Failed in QUIC accept"); ce != nil {
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
			if ce := utils.CanLogErr("Failed in QUIC early accept"); ce != nil {
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
			if ce := utils.CanLogDebug("Failed in QUIC stream accept"); ce != nil {
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
		theChan <- &StreamConn{stream, conn.LocalAddr(), conn.RemoteAddr(), nil, false}
	}
}

//implements advLayer.SuperMuxServer
type Server struct {
	Creator

	addr    string
	tlsConf tls.Config
	args    arguments

	listener io.Closer
}

func (s *Server) GetPath() string {
	return ""
}

func (s *Server) Stop() {

	if s.listener != nil {
		s.listener.Close()

	}

}

func (s *Server) StartListen() (newSubConnChan chan net.Conn, baseConn io.Closer) {

	newSubConnChan, baseConn = ListenInitialLayers(s.addr, s.tlsConf, s.args)
	if baseConn != nil {
		s.listener = baseConn

	}
	return
}

//非阻塞，不支持回落。
func (s *Server) StartHandle(underlay net.Conn, newSubConnChan chan net.Conn, fallbackConnChan chan httpLayer.FallbackMeta) {
	go dealNewConn(underlay.(quic.Connection), newSubConnChan)
}
