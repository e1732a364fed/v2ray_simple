package shadowsocks

import (
	"bytes"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

type shadowUDPPacketConn struct {
	net.PacketConn
	raddr net.Addr
	taddr net.Addr

	handshakeBuf *bytes.Buffer
}

func (c *shadowUDPPacketConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return c.PacketConn.Close()
}

func (c *shadowUDPPacketConn) Fullcone() bool {
	return true
}

func (c *shadowUDPPacketConn) ReadMsgFrom() (bs []byte, targetAddr netLayer.Addr, err error) {
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

func (c *shadowUDPPacketConn) WriteMsgTo(bs []byte, addr netLayer.Addr) (err error) {
	var buf *bytes.Buffer

	if c.handshakeBuf != nil {
		buf = c.handshakeBuf
		c.handshakeBuf = nil

	} else {
		buf = utils.GetBuf()

	}

	abs, atype := addr.AddressBytes()

	atype = netLayer.ATypeToSocks5Standard(atype)

	buf.WriteByte(atype)
	buf.Write(abs)

	buf.WriteByte(byte(addr.Port >> 8))
	buf.WriteByte(byte(addr.Port << 8 >> 8))

	buf.WriteByte(byte(len(bs) >> 8))
	buf.WriteByte(byte(len(bs) << 8 >> 8))
	buf.Write(bs)

	_, err = c.PacketConn.WriteTo(buf.Bytes(), c.raddr)
	utils.PutBuf(buf)

	return err
}
