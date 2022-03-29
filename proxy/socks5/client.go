package socks5

import (
	"bytes"
	"errors"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//为了安全, 我们不支持socks5作为 proxy.Client;
// 不过为了测试 udp associate 需要我们需要模拟一下socks5请求

func Client_EstablishUDPAssociate(conn net.Conn) (port int, err error) {
	var ba [10]byte

	//握手阶段
	ba[0] = Version5
	ba[1] = 1
	ba[2] = 0
	_, err = conn.Write(ba[:3])
	if err != nil {
		return
	}

	n, err := conn.Read(ba[:])
	if err != nil {
		return
	}
	if n != 2 || ba[0] != Version5 || ba[1] != 0 {
		return 0, utils.NumErr{Prefix: "EstablishUDPAssociate,protocol err", N: 1}
	}

	//请求udp associate 阶段

	ba[0] = Version5
	ba[1] = CmdUDPAssociate
	ba[2] = 0
	ba[3] = ATypIP4
	ba[4] = 0
	ba[5] = 0
	ba[6] = 0
	ba[7] = 0
	ba[8] = 0 //port
	ba[9] = 0 //port
	// 按理说要告诉服务端我们要用到的ip和端口，但是我们不知道，所以全填零
	// 在内网中的话，我们是可以知道的，但是因为内网很安全所以无所谓；在NAT中我们肯定是不知道的。
	// 如果是在纯外网中则是可以知道的，但是为啥非要socks5这么不安全的协议呢？所以还是不予考虑。

	_, err = conn.Write(ba[:10])
	if err != nil {
		return
	}

	n, err = conn.Read(ba[:])
	if err != nil {
		return
	}
	if n != 10 || ba[0] != Version5 || ba[1] != 0 || ba[2] != 0 || ba[3] != 1 || ba[4] != 0 || ba[5] != 0 || ba[6] != 0 || ba[7] != 0 {
		return 0, utils.NumErr{Prefix: "EstablishUDPAssociate,protocol err", N: 2}
	}

	port = int(ba[8])<<8 | int(ba[9])
	return

}

// RequestUDP 向一个 socks5服务器监听的 udp端口发送一次udp请求
//在udp associate结束后，就已经知道了服务器给我们专用的port了，向这个端口发送一个udp请求.
//
// 另外的备忘是, 服务器返回的数据使用了相同的结构
func Client_RequestUDP(udpConn *net.UDPConn, target *netLayer.Addr, data []byte) {

	if udpConn == nil {
		//不允许
	}

	buf := &bytes.Buffer{}
	buf.WriteByte(0)
	buf.WriteByte(0)
	buf.WriteByte(0)

	abs, atype := target.AddressBytes()
	switch atype {
	case 0:
		//无效地址

	case netLayer.AtypIP4:
		buf.WriteByte(ATypIP4)
	case netLayer.AtypIP6:
		buf.WriteByte(ATypIP6)

	case netLayer.AtypDomain:
		buf.WriteByte(ATypDomain)

	}
	buf.Write(abs)

	port := target.Port

	buf.WriteByte(byte(int16(port) >> 8))
	buf.WriteByte(byte(int16(port) << 8 >> 8))

	buf.Write(data)

	udpConn.Write(buf.Bytes())
}

//从 一个 socks5服务器的udp端口 读取一次 udp回应
func Client_ReadUDPResponse(udpConn *net.UDPConn, supposedServerAddr *net.UDPAddr) (target netLayer.Addr, data []byte, e error) {

	buf := utils.GetPacket()
	n, addr, err := udpConn.ReadFromUDP(buf)
	//log.Println("Client_ReadUDPResponse, got data")

	if err != nil {
		e = err
		return
	}
	if n < 6 {
		e = errors.New("UDPConn short read err")
		return
	}

	if !(addr.IP.Equal(supposedServerAddr.IP) && addr.Port == supposedServerAddr.Port) {
		e = utils.NewDataErr("socks5 Client_ReadUDPResponse , got data from unknown source", nil, addr)
		return
	}
	if buf[0] != 0 || buf[1] != 0 || buf[2] != 0 {
		e = utils.NumErr{Prefix: "EstablishUDPAssociate,protocol err", N: 1}
		return
	}
	atype := buf[3]
	remainBuf := bytes.NewBuffer(buf[4:n])

	switch atype {
	case ATypIP4:
		ipbs := make([]byte, 4)
		remainBuf.Read(ipbs)
		target.IP = ipbs
	case ATypIP6:
		ipbs := make([]byte, net.IPv6len)
		remainBuf.Read(ipbs)
		target.IP = ipbs
	case ATypDomain:
		nameLen, _ := remainBuf.ReadByte()
		nameBuf := make([]byte, nameLen)
		remainBuf.Read(nameBuf)

		target.Name = string(nameBuf)

	default:
		e = utils.NumErr{Prefix: "EstablishUDPAssociate,protocol err", N: 2}
		return
	}

	pb1, _ := remainBuf.ReadByte()
	pb2, _ := remainBuf.ReadByte()

	target.Port = int(pb1)<<8 | int(pb2)

	data = remainBuf.Bytes()

	return

}
