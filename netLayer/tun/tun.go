package tun

import (
	"io"
	"log"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/tun"
	"github.com/songgao/water"
	"go.uber.org/zap"
)

type coreUDPConnAdapter struct {
	core.UDPConn
	netLayer.EasyDeadline
}

func (h *coreUDPConnAdapter) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	return nil, netLayer.Addr{}, nil
}
func (h *coreUDPConnAdapter) WriteMsgTo([]byte, netLayer.Addr) error {
	return nil
}
func (h *coreUDPConnAdapter) CloseConnWithRaddr(raddr netLayer.Addr) error {

	return nil
}
func (h *coreUDPConnAdapter) Fullcone() bool {
	return true
}

type handler struct {
	tcpChan chan netLayer.TCPRequestInfo
	udpChan chan netLayer.UDPRequestInfo
}

func newHandler() *handler {
	return &handler{
		tcpChan: make(chan netLayer.TCPRequestInfo),
		udpChan: make(chan netLayer.UDPRequestInfo),
	}
}

func (h *handler) Handle(conn net.Conn, target *net.TCPAddr) error {
	tad := netLayer.NewAddrFromTCPAddr(target)
	//实测 这里 target 就是 conn.RemoteAddr()

	h.tcpChan <- netLayer.TCPRequestInfo{Target: tad, Conn: conn}
	return nil
}

func (h *handler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	uad := netLayer.NewAddrFromUDPAddr(target)
	adapter := &coreUDPConnAdapter{UDPConn: conn}
	adapter.InitEasyDeadline()

	h.udpChan <- netLayer.UDPRequestInfo{Target: uad, MsgConn: adapter}
	return nil
}

func (h *handler) ReceiveTo(conn core.UDPConn, data []byte, addr *net.UDPAddr) error {
	log.Println("ReceiveTo called")
	return nil
}

func ListenTun() (tunDev io.ReadWriteCloser, err error) {

	//macos 上无法指定tun名称
	tunDev, err = tun.OpenTunDevice("", "10.1.0.10", "10.1.0.20", "255.255.255.0", nil, false)
	if err == nil {
		if ce := utils.CanLogInfo("created new tun device"); ce != nil {
			ce.Write(zap.String("name", tunDev.(*water.Interface).Name()))
		}
	}
	/*
		如果你是 tun listen, direct dial,
		要配置好路由表；如果默认情况的话，会造成无限本地回环，因为我们代理发出的流量又被导向代理本身


	*/

	return
}

func HandleTun(tunDev io.ReadWriteCloser) (tcpChan <-chan netLayer.TCPRequestInfo, udpChan <-chan netLayer.UDPRequestInfo, closer io.Closer) {
	lwip := core.NewLWIPStack()
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})
	nh := newHandler()
	core.RegisterTCPConnHandler(nh)
	core.RegisterUDPConnHandler(nh)
	go func() {
		_, err := io.CopyBuffer(lwip, tunDev, make([]byte, utils.MTU))
		if err != nil {
			log.Fatalf("tun copying from tunDev to lwip failed: %v", err)
		}
	}()
	tcpChan = nh.tcpChan
	udpChan = nh.udpChan
	closer = lwip
	return
}
