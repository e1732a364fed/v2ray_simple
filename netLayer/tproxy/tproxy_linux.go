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
		IP:      targetTCPAddr.IP,
		Port:    targetTCPAddr.Port,
		Network: "tcp",
	}

}

func (m *Machine) removeUDPByHash(hash netLayer.HashableAddr) {
	m.Lock()
	delete(m.udpMsgConnMap, hash)
	m.Unlock()
}

//从一个透明代理udp连接中读取到实际地址，并返回 *MsgConn
func (m *Machine) HandshakeUDP(underlay *net.UDPConn) (*MsgConn, netLayer.Addr, error) {

	for {
		bs := utils.GetPacket()
		n, src, dst, err := ReadFromUDP(underlay, bs)
		if err != nil {
			return nil, netLayer.Addr{}, err
		}
		ad := netLayer.NewAddrFromUDPAddr(src)
		hash := ad.GetHashable()

		m.RLock()
		conn, found := m.udpMsgConnMap[hash]
		m.RUnlock()

		if !found {
			conn = &MsgConn{
				ourSrcAddr:    src,
				readChan:      make(chan netLayer.AddrData, 5),
				closeChan:     make(chan struct{}),
				parentMachine: m,
				hash:          hash,
			}
			conn.InitEasyDeadline()

			m.Lock()
			m.udpMsgConnMap[hash] = conn
			m.Unlock()

		}

		destAddr := netLayer.NewAddrFromUDPAddr(dst)

		conn.readChan <- netLayer.AddrData{Data: bs[:n], Addr: destAddr}

		if !found {
			return conn, destAddr, nil

		}

	}

}

func (mc *MsgConn) Close() error {
	select {
	case <-mc.closeChan:
	default:
		close(mc.closeChan)
		mc.parentMachine.removeUDPByHash(mc.hash)

	}
	return nil
}

func (mc *MsgConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return mc.Close()
}

func (mc *MsgConn) Fullcone() bool {
	return mc.fullcone
}

func (mc *MsgConn) SetFullcone(f bool) {
	mc.fullcone = f
}

func (mc *MsgConn) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	must_timeoutChan := time.After(netLayer.UDP_timeout)
	select {
	case <-mc.closeChan:
		return nil, netLayer.Addr{}, io.EOF
	case <-must_timeoutChan:
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
