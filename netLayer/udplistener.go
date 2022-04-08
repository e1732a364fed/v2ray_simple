package netLayer

import (
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/hahahrfool/v2ray_simple/utils"
)

// UDPListener 实现了 net.Listener.
// UDPListener 监听 UDPAddr，并不断对新远程地址 创建 新UDPConn并提供给Accept;
// 然后读到的信息缓存到 UDPConn 中，让它能在Read时读到.
//
//UDPListener can also dial a remote host by calling NewConn.
type UDPListener struct {
	conn *net.UDPConn

	newConnChan chan *Uni_UDPConn
	connMap     map[netip.AddrPort]*Uni_UDPConn
	mux         sync.RWMutex
	isclosed    bool
}

// NewUDPListener 返回一个 *UDPListener, 该Listener实现了 net.Listener
func NewUDPListener(laddr *net.UDPAddr) (*UDPListener, error) {
	c, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return nil, err
	}
	return NewUDPListenerConn(c)
}

func NewUDPListenerConn(conn *net.UDPConn) (*UDPListener, error) {
	ul := new(UDPListener)
	ul.conn = conn
	ul.connMap = make(map[netip.AddrPort]*Uni_UDPConn)
	ul.newConnChan = make(chan *Uni_UDPConn, 100)
	go ul.run()

	return ul, nil
}

//It can be used to dial a remote udp
func (ul *UDPListener) NewConn(raddr *net.UDPAddr) *Uni_UDPConn {
	return ul.newConn(raddr, UDPAddr2AddrPort(raddr))
}

//newConn 创建一个新的 UDPConn,并存储在 ul.connMap 中
func (ul *UDPListener) newConn(raddr *net.UDPAddr, addrport netip.AddrPort) *Uni_UDPConn {
	newC := NewUDPConn(raddr, ul.conn, false)
	ul.mux.Lock()
	ul.connMap[addrport] = newC
	ul.mux.Unlock()
	return newC
}

func (ul *UDPListener) DeleteConn(addrport netip.AddrPort) {
	ul.mux.Lock()
	delete(ul.connMap, addrport)
	ul.mux.Unlock()
}

func (ul *UDPListener) Accept() (net.Conn, error) {
	c, ok := <-ul.newConnChan
	if !ok {
		return nil, io.EOF
	}
	return c, nil
}

//Once closed, it cannot be used again.
// it calls ul.CloseClients()
func (ul *UDPListener) Close() error {
	if ul.isclosed {
		return nil
	}

	ul.isclosed = true

	err := ul.conn.Close()
	if err != nil {
		return err
	}

	ul.closeClients()

	return nil
}

//UDPListener has a very fast way to close all the clients' connection.
//		If the server side of the client connection is reading, it will get an EOF error.
//		The application can then tell the remote client that it will be closed by sending a message using its custom protocol.
//
//Once closed, it cannot be used again.
func (ul *UDPListener) closeClients() error {
	close(ul.newConnChan)

	ul.mux.Lock()
	for _, c := range ul.connMap {
		close(c.inMsgChan)
	}
	ul.connMap = make(map[netip.AddrPort]*Uni_UDPConn)
	ul.mux.Unlock()

	return nil
}

func (ul *UDPListener) Addr() net.Addr {
	return ul.conn.LocalAddr()
}

//循环读取udp数据，对新连接会创建 UDPConn，然后把数据通过chan 传递给UDPConn
func (ul *UDPListener) run() {
	conn := ul.conn
	for {
		buf := utils.GetPacket()
		n, raddr, err := conn.ReadFromUDP(buf)

		go func(theraddr *net.UDPAddr, thebuf []byte) {
			addrport := UDPAddr2AddrPort(theraddr)
			var oldConn *Uni_UDPConn

			ul.mux.RLock()
			oldConn = ul.connMap[addrport]
			ul.mux.RUnlock()

			if oldConn == nil {
				oldConn = ul.newConn(raddr, addrport)

				ul.newConnChan <- oldConn //此时 ul 的 Accept的调用者就会收到一个新Conn
			}

			oldConn.inMsgChan <- thebuf[:n]

		}(raddr, buf)

		if err != nil {
			return
		}

	}
}
