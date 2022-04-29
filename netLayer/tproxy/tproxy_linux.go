package tproxy

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

//从一个透明代理tcp连接中读取到实际地址
func HandshakeTCP(tcpConn *net.TCPConn) netLayer.Addr {
	targetTCPAddr := tcpConn.LocalAddr().(*net.TCPAddr)

	return netLayer.Addr{
		IP:   targetTCPAddr.IP,
		Port: targetTCPAddr.Port,
	}

}

var udpMsgConnMap = make(map[netLayer.HashableAddr]*MsgConn)

//从一个透明代理udp连接中读取到实际地址，并返回 *MsgConn
func HandshakeUDP(underlay *net.UDPConn) (netLayer.MsgConn, netLayer.Addr, error) {
	bs := utils.GetPacket()
	n, src, dst, err := ReadFromUDP(underlay, bs)
	if err != nil {
		return nil, netLayer.Addr{}, err
	}
	ad := netLayer.NewAddrFromUDPAddr(src)
	hash := ad.GetHashable()
	conn, found := udpMsgConnMap[hash]
	if !found {
		conn = &MsgConn{
			ourSrcAddr: src,
			readChan:   make(chan netLayer.AddrData, 5),
			closeChan:  make(chan struct{}),
		}

		udpMsgConnMap[hash] = conn

	}

	conn.readChan <- netLayer.AddrData{Data: bs[:n], Addr: netLayer.NewAddrFromUDPAddr(dst)}

	return conn, netLayer.NewAddrFromUDPAddr(dst), nil
}

//implements netLayer.MsgConn
type MsgConn struct {
	ourSrcAddr *net.UDPAddr

	readChan chan netLayer.AddrData

	closeChan chan struct{}
}

func (mc *MsgConn) Close() error {
	select {
	case <-mc.closeChan:
	default:
		close(mc.closeChan)
	}
	return nil
}

func (mc *MsgConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return nil
}

func (mc *MsgConn) Fullcone() bool {
	return true
}

func (mc *MsgConn) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	timeoutChan := time.After(netLayer.UDP_timeout)
	select {
	case <-mc.closeChan:
		return nil, netLayer.Addr{}, io.EOF
	case <-timeoutChan:
		return nil, netLayer.Addr{}, os.ErrDeadlineExceeded
	case newmsg := <-mc.readChan:
		return newmsg.Data, newmsg.Addr, nil

	}

}

// 通过透明代理 写回 客户端
func (mc *MsgConn) WriteMsgTo(p []byte, addr netLayer.Addr) error {
	back, err := DialUDP(
		"udp",
		&net.UDPAddr{
			IP:   addr.IP,
			Port: addr.Port,
		},
		mc.ourSrcAddr,
	)
	if err != nil {
		return err
	}
	_, err = back.Write(p)
	if err != nil {

		return err
	}
	back.Close()
	return nil
}
