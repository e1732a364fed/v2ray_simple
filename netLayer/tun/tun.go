package tun

import (
	"io"
	"log"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/tun"
)

type coreConnAdapter struct {
	core.UDPConn
	netLayer.EasyDeadline
}

func (h *coreConnAdapter) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	return nil, netLayer.Addr{}, nil
}
func (h *coreConnAdapter) WriteMsgTo([]byte, netLayer.Addr) error {
	return nil
}
func (h *coreConnAdapter) CloseConnWithRaddr(raddr netLayer.Addr) error {

	return nil
}
func (h *coreConnAdapter) Fullcone() bool {
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
	h.tcpChan <- netLayer.TCPRequestInfo{Target: tad, Conn: conn}
	return nil
}

func (h *handler) Connect(conn core.UDPConn, target *net.UDPAddr) error {
	uad := netLayer.NewAddrFromUDPAddr(target)
	adapter := &coreConnAdapter{UDPConn: conn}
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

	return
}

func HandleTun(tunDev io.ReadWriteCloser) (tcpChan chan netLayer.TCPRequestInfo, udpChan chan netLayer.UDPRequestInfo) {
	lwipWriter := core.NewLWIPStack().(io.Writer)
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})
	nh := newHandler()
	core.RegisterTCPConnHandler(nh)
	core.RegisterUDPConnHandler(nh)
	go func() {
		_, err := io.CopyBuffer(lwipWriter, tunDev, make([]byte, utils.MTU))
		if err != nil {
			log.Fatalf("tun copying from tunDev to lwip failed: %v", err)
		}
	}()
	tcpChan = nh.tcpChan
	udpChan = nh.udpChan
	return
}
