package shadowsocks

import (
	"bytes"
	"io"
	"net"
	"os"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

type clientUDPMsgConn struct {
	net.PacketConn
	raddr net.Addr
}

func (c *clientUDPMsgConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return c.PacketConn.Close()
}

func (c *clientUDPMsgConn) Fullcone() bool {
	return true
}

func (c *clientUDPMsgConn) ReadMsg() (bs []byte, targetAddr netLayer.Addr, err error) {
	buf := utils.GetPacket()

	var n int
	n, _, err = c.PacketConn.ReadFrom(buf)
	if err != nil {
		return
	}

	readbuf := bytes.NewBuffer(buf[:n])

	targetAddr, err = GetAddrFrom(readbuf)
	if err != nil {
		return
	}
	bs = readbuf.Bytes()

	return

}

func makeWriteBuf(bs []byte, addr netLayer.Addr) *bytes.Buffer {
	buf := utils.GetBuf()

	abs, atype := addr.AddressBytes()

	atype = netLayer.ATypeToSocks5Standard(atype)

	buf.WriteByte(atype)
	buf.Write(abs)

	buf.WriteByte(byte(addr.Port >> 8))
	buf.WriteByte(byte(addr.Port << 8 >> 8))

	buf.WriteByte(byte(len(bs) >> 8))
	buf.WriteByte(byte(len(bs) << 8 >> 8))
	buf.Write(bs)

	return buf
}

func (c *clientUDPMsgConn) WriteMsg(bs []byte, addr netLayer.Addr) (err error) {

	buf := makeWriteBuf(bs, addr)

	_, err = c.PacketConn.WriteTo(buf.Bytes(), c.raddr)
	utils.PutBuf(buf)

	return err
}

// implements netLayer.serverMsgConn, 完全类似tproxy
type serverMsgConn struct {
	netLayer.EasyDeadline

	hash netLayer.HashableAddr

	ourPacketConn net.PacketConn
	raddr         net.Addr

	readChan chan netLayer.AddrData

	closeChan chan struct{}

	fullcone bool

	server *Server
}

func (mc *serverMsgConn) Close() error {
	select {
	case <-mc.closeChan:
	default:
		close(mc.closeChan)
		mc.server.removeUDPByHash(mc.hash)

	}
	return nil
}

func (mc *serverMsgConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return mc.Close()
}

func (mc *serverMsgConn) Fullcone() bool {
	return mc.fullcone
}

func (mc *serverMsgConn) SetFullcone(f bool) {
	mc.fullcone = f
}

func (mc *serverMsgConn) ReadMsg() ([]byte, netLayer.Addr, error) {

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

func (mc *serverMsgConn) WriteMsg(p []byte, addr netLayer.Addr) error {
	buf := makeWriteBuf(p, addr)
	_, err := mc.ourPacketConn.WriteTo(buf.Bytes(), mc.raddr)

	return err
}
