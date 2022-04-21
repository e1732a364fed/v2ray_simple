package socks5

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

// 解读如下：
//ver（5）, rep（0，表示成功）, rsv（0）, atyp(1, 即ipv4), BND.ADDR （ipv4(0,0,0,0)）, BND.PORT(0, 2字节)
//这个 BND.ADDR和port 按理说不应该传0的，不过如果只作为本地tcp代理的话应该不影响
var commmonTCP_HandshakeReply = []byte{Version5, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

// TODO: Support Auth
type Server struct {
	proxy.ProxyCommonStruct
	//user string
	//password string
}

type ServerCreator struct{}

func (ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	s := &Server{}
	return s, nil
}

func (ServerCreator) NewServer(dc *proxy.ListenConf) (proxy.Server, error) {
	s := &Server{}
	return s, nil
}

func (*Server) Name() string { return Name }

// 处理tcp收到的请求. 注意, udp associate后的 udp请求并不通过此函数处理, 而是由 UDPConn 处理
func (*Server) Handshake(underlay net.Conn) (result net.Conn, udpChannel netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	// Set handshake timeout 4 seconds
	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		returnErr = err
		return
	}
	defer underlay.SetReadDeadline(time.Time{})

	buf := utils.GetMTU()
	defer utils.PutBytes(buf)

	// Read hello message
	// 一般握手包发来的是 [5 1 0]
	n, err := underlay.Read(buf)
	if err != nil || n == 0 {
		returnErr = fmt.Errorf("failed to read hello: %w", err)
		return
	}
	version := buf[0]
	if version != Version5 {
		returnErr = fmt.Errorf("unsupported socks version %v", version)
		return
	}

	// Write hello response， [5 0]
	// TODO: Support Auth
	_, err = underlay.Write([]byte{Version5, AuthNone})
	if err != nil {
		returnErr = fmt.Errorf("failed to write hello response: %w", err)
		return
	}

	// Read command message，
	n, err = underlay.Read(buf)
	if err != nil || n < 7 { // Shortest length is 7
		returnErr = fmt.Errorf("read socks5 failed, msgTooShort: %w", err)
		return
	}

	// 一般可以为 5 1 0 3 n，3表示域名，n是域名长度，然后域名很可能是 119 119 119 46 开头，表示 www.
	//  比如百度就是  [5 1 0 3 13 119 119 119 46 98]

	cmd := buf[1]
	if cmd == CmdBind {
		returnErr = fmt.Errorf("unsuppoted command %v", cmd)
		return
	}

	l := 2
	off := 4
	var theIP net.IP

	switch buf[3] {
	case ATypIP4:
		l += net.IPv4len
		theIP = make(net.IP, net.IPv4len)
	case ATypIP6:
		l += net.IPv6len
		theIP = make(net.IP, net.IPv6len)
	case ATypDomain:
		l += int(buf[4])
		off = 5
	default:
		returnErr = fmt.Errorf("unknown address type %v", buf[3])
		return
	}

	if len(buf[off:]) < l {
		returnErr = errors.New("short command request")
		return
	}

	var theName string

	if theIP != nil {
		copy(theIP, buf[off:])
	} else {
		theName = string(buf[off : off+l-2])
	}
	var thePort int
	thePort = int(buf[off+l-2])<<8 | int(buf[off+l-1])

	//根据 socks5标准，“使用UDP ASSOCIATE时，客户端的请求包中(DST.ADDR, DST.PORT)不再是目标的地址，而是客户端指定本身用于发送UDP数据包的地址和端口”
	//然后服务器会传回专门适用于客户端的 一个 服务器的 udp的ip和地址；然后之后客户端再专门 向udp地址发送连接，此tcp连接就已经没用。
	//总之，UDP Associate方法并不是 UDP over TCP，完全不同，而且过程中握手用tcp，传输用udp，使用到了两个连接。

	//不过一般作为NAT内网中的客户端是无法知道自己实际呈现给服务端的udp地址的, 所以没什么太大用
	// 但是, 如果不指定的话，udp associate 就会认为未来一切发往我们新生成的端口的连接都是属于该客户端, 显然有风险
	// 更不用说这样 的 udp associate 会重复使用很多 随机udp端口，特征很明显。
	// 总之 udp associate 只能用于内网环境。

	if cmd == CmdUDPAssociate {

		//这里我们serverAddr直接返回0.0.0.0即可，也实在想不到谁会返回 另一个ip地址出来。肯定应该和原ip相同的。

		//随机生成一个端口专门用于处理该客户端。这是我的想法。

		bindPort := netLayer.RandPort(true, true)

		udpPreparedAddr := &net.UDPAddr{
			IP:   []byte{0, 0, 0, 0},
			Port: bindPort,
		}

		udpRC, err := net.ListenUDP("udp", udpPreparedAddr)
		if err != nil {
			returnErr = errors.New("UDPAssociate: unable to listen udp")
			return
		}

		//ver（5）, rep（0，表示成功）, rsv（0）, atyp(1, 即ipv4), BND.ADDR(4字节的ipv4) , BND.PORT(2字节)
		reply := [10]byte{Version5, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(int16(bindPort) >> 8), byte(int16(bindPort) << 8 >> 8)}

		_, err = underlay.Write(reply[:])
		if err != nil {
			returnErr = fmt.Errorf("failed to write command response: %w", err)
			return
		}

		//theName有可能是ip的形式，比如浏览器一般不会自己dns，把一切ip和域名都当作域名传入socks5代理
		if ip := net.ParseIP(theName); ip != nil {
			theIP = ip
		}

		clientFutureAddr := netLayer.Addr{
			IP:      theIP,
			Name:    theName,
			Port:    thePort,
			Network: "udp",
		}

		uc := &ServerUDPConn{
			clientSupposedAddr: clientFutureAddr.ToUDPAddr(), //这里为了解析域名, 就用了 netLayer.Addr 作为中介的方式
			UDPConn:            udpRC,
		}
		return nil, uc, clientFutureAddr, nil

	} else {

		_, err = underlay.Write(commmonTCP_HandshakeReply)
		if err != nil {
			returnErr = fmt.Errorf("failed to write command response: %w", err)
			return
		}

		//theName有可能是ip的形式，比如浏览器一般不会自己dns，把一切ip和域名都当作域名传入socks5代理

		if ip := net.ParseIP(theName); ip != nil {
			theIP = ip
		}

		targetAddr = netLayer.Addr{
			IP:   theIP,
			Name: theName,
			Port: thePort,
		}

		return underlay, nil, targetAddr, nil
	}

}

//用于socks5服务端的 udp连接, 实现 netLayer.MsgConn
type ServerUDPConn struct {
	*net.UDPConn
	clientSupposedAddr *net.UDPAddr //客户端指定的客户端自己未来将使用的公网UDP的Addr
}

func (u *ServerUDPConn) CloseConnWithRaddr(raddr netLayer.Addr) error {
	return u.Close()
}

func (u *ServerUDPConn) Fullcone() bool {
	return true
}

//将远程地址发来的响应 传给客户端
func (u *ServerUDPConn) WriteMsgTo(bs []byte, raddr netLayer.Addr) error {

	buf := &bytes.Buffer{}
	buf.WriteByte(0) //rsv
	buf.WriteByte(0) //rsv
	buf.WriteByte(0) //frag

	var atyp byte = ATypIP4
	if len(raddr.IP) > 4 {
		atyp = ATypIP6
	}
	buf.WriteByte(atyp)

	buf.Write(raddr.IP)
	buf.WriteByte(byte(int16(raddr.Port) >> 8))
	buf.WriteByte(byte(int16(raddr.Port) << 8 >> 8))
	buf.Write(bs)

	//必须要指明raddr
	_, err := u.UDPConn.WriteToUDP(buf.Bytes(), u.clientSupposedAddr)

	return err

}

//从 客户端读取 udp请求
func (u *ServerUDPConn) ReadMsgFrom() ([]byte, netLayer.Addr, error) {

	var clientSupposedAddrIsNothing bool
	if len(u.clientSupposedAddr.IP) < 3 || u.clientSupposedAddr.IP.IsUnspecified() {
		clientSupposedAddrIsNothing = true
	}

	bs := utils.GetPacket()

	n, addr, err := u.UDPConn.ReadFromUDP(bs)
	if err != nil {

		if n <= 0 {
			return nil, netLayer.Addr{}, err
		}
		return bs[:n], netLayer.NewAddrFromUDPAddr(addr), err
	}

	if n < 6 {

		return nil, netLayer.Addr{}, utils.ErrInErr{ErrDesc: "socks5 UDPConn short read", Data: n}
	}

	if !clientSupposedAddrIsNothing {

		if !addr.IP.Equal(u.clientSupposedAddr.IP) || addr.Port != u.clientSupposedAddr.Port {

			//just random attack message.
			return nil, netLayer.Addr{}, utils.ErrInErr{ErrDesc: "socks5 UDPConn ReadMsg failed, addr not coming from supposed client addr", ErrDetail: utils.ErrInvalidData, Data: addr.String()}

		}
	}

	atyp := bs[3]

	l := 2   //supposed Minimum Remain Data Lenth
	off := 4 //offset from which the addr data really starts

	var theIP net.IP
	switch atyp {
	case ATypIP4:
		l += net.IPv4len
		theIP = make(net.IP, net.IPv4len)
	case ATypIP6:
		l += net.IPv6len
		theIP = make(net.IP, net.IPv6len)
	case ATypDomain:
		l += int(bs[4])
		off = 5
	default:

		return nil, netLayer.Addr{}, utils.ErrInErr{ErrDesc: "socks5 read UDPConn unknown atype", Data: atyp}

	}

	if len(bs[off:]) < l {

		return nil, netLayer.Addr{}, utils.ErrInErr{ErrDesc: "socks5 UDPConn short command request", Data: atyp}

	}

	var theName string

	if theIP != nil {
		copy(theIP, bs[off:])
	} else {
		theName = string(bs[off : off+l-2])
	}
	var thePort int
	thePort = int(bs[off+l-2])<<8 | int(bs[off+l-1])

	newStart := off + l

	if clientSupposedAddrIsNothing {
		u.clientSupposedAddr = addr
	}

	return bs[newStart:n], netLayer.Addr{
		IP:      theIP,
		Name:    theName,
		Port:    thePort,
		Network: "udp",
	}, nil
}
