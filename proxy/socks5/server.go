package socks5

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"time"

	"github.com/hahahrfool/v2ray_simple/config"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

func init() {
	proxy.RegisterServerWithURL(Name, &ServerCreator{})
}

type Server struct {
	proxy.ProxyCommonStruct
	//user string
	//password string
}

//只有地址和port需要配资，非常简单
type ServerCreator struct{}

func (_ ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return NewServer(u)
}

func (_ ServerCreator) NewServer(dc *config.ListenConf, m map[string]interface{}) (proxy.Server, error) {

	s := &Server{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: dc.GetAddr()},
	}
	return s, nil
}

func NewServer(url *url.URL) (proxy.Server, error) {
	addr := url.Host //若不给出port，那就只有host名，这样不好，我们默认配置里肯定给了port

	// TODO: Support Auth

	s := &Server{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: addr},
	}
	s.ProxyCommonStruct.InitFromUrl(url)
	return s, nil
}

func (s *Server) Name() string { return Name }

func (s *Server) CanFallback() bool {
	return false
}

//English: https://www.ietf.org/rfc/rfc1928.txt

//中文： https://aber.sh/articles/Socks5/
// 参考 https://studygolang.com/articles/31404

func (s *Server) Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error) {
	// Set handshake timeout 4 seconds
	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		return nil, nil, err
	}
	defer underlay.SetReadDeadline(time.Time{})

	buf := utils.GetPacket()
	defer utils.PutPacket(buf)

	// Read hello message
	// 一般握手包发来的是 [5 1 0]
	n, err := underlay.Read(buf)
	if err != nil || n == 0 {
		return nil, nil, fmt.Errorf("failed to read hello: %w", err)
	}
	version := buf[0]
	if version != Version5 {
		return nil, nil, fmt.Errorf("unsupported socks version %v", version)
	}

	// Write hello response， [5 0]
	// TODO: Support Auth
	_, err = underlay.Write([]byte{Version5, AuthNone})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to write hello response: %w", err)
	}

	// Read command message，
	n, err = underlay.Read(buf)
	if err != nil || n < 7 { // Shortest length is 7
		return nil, nil, fmt.Errorf("read socks5 failed, msgTooShort: %w", err)
	}

	// 一般可以为 5 1 0 3 n，3表示域名，n是域名长度，然后域名很可能是 119 119 119 46 开头，表示 www.
	//  比如百度就是  [5 1 0 3 13 119 119 119 46 98]

	cmd := buf[1]
	switch cmd {
	case CmdBind:
		return nil, nil, fmt.Errorf("unsuppoted command %v", cmd)
	case CmdUDPAssociate:

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
		return nil, nil, fmt.Errorf("unknown address type %v", buf[3])
	}

	if len(buf[off:]) < l {
		return nil, nil, errors.New("short command request")
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

	if cmd == CmdUDPAssociate {
		clientFutureAddr := &netLayer.Addr{
			IP:    theIP,
			Name:  theName,
			Port:  thePort,
			IsUDP: true,
		}

		serverAtyp, serverAddr, _, err := netLayer.ParseStrToAddr(s.Addr)
		if serverAtyp != netLayer.AtypIP4 { //暂时先只支持ipv4，为了简单起见
			if err != nil {
				return nil, nil, errors.New("UDPAssociate: can't listen an domain, must be ip")
			}
		}

		//随机生成一个端口专门用于处理该客户端。这是我的想法。

		bindPort := 1024 + rand.Intn(50000)

		udpPreparedAddr := &net.UDPAddr{
			IP:   serverAddr,
			Port: bindPort,
		}

		udpRC, err := net.ListenUDP("udp", udpPreparedAddr)
		if err != nil {
			return nil, nil, errors.New("UDPAssociate: unable to listen udp")
		}

		//ver（5）, rep（0，表示成功）, rsv（0）, atyp(1, 即ipv4), BND.ADDR(4字节的ipv4) , BND.PORT(2字节)
		reply := []byte{Version5, 0x00, 0x00, 0x01, serverAddr[0], serverAddr[1], serverAddr[2], serverAddr[3], byte(int16(bindPort) >> 8), byte(int16(bindPort) << 8 >> 8)}

		// Write command response
		_, err = underlay.Write(reply)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to write command response: %w", err)
		}

		uc := &UDPConn{
			clientSupposedAddr: clientFutureAddr,
			UDPConn:            udpRC,
		}
		return uc, clientFutureAddr, nil

	} else {
		addr := &netLayer.Addr{
			IP:   theIP,
			Name: theName,
			Port: thePort,
		}

		// 解读如下：
		//ver（5）, rep（0，表示成功）, rsv（0）, atyp(1, 即ipv4), BND.ADDR （ipv4(0,0,0,0)）, BND.PORT(0, 2字节)
		reply := []byte{Version5, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

		//这个 BND.ADDR和port 按理说不应该传0的，不过如果只作为本地tcp代理的话应该不影响

		_, err = underlay.Write(reply)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to write command response: %w", err)
		}

		return underlay, addr, nil
	}

}

func (s *Server) Stop() {
	// Nothing to stop or close
}

type UDPConn struct {
	*net.UDPConn
	clientSupposedAddr *netLayer.Addr //客户端指定的客户端自己未来将使用的公网UDP的Addr

	clientSupposedAddrIsNothing bool
}

// 阻塞
// 从 udpPutter.GetNewUDPResponse 循环阅读 所有需要发送给客户端的 数据，然后发送给客户端
//  这些响应数据是 由其它设施 写入 udpProxy的
func (u *UDPConn) StartPushResponse(udpPutter proxy.UDP_Putter) {
	for {
		raddr, bs, err := udpPutter.GetNewUDPResponse()
		if err != nil {
			break
		}
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

		_, err = u.UDPConn.Write(buf.Bytes())
		if err != nil {
			break
		}
	}
}

// 阻塞
// 监听 与客户端的udp连接；循环查看客户端发来的请求信息; 然后将该请求 用 udpPutter.WriteUDPRequest 发送给 udpPutter
//	至于fullcone与否它是不管的。
func (u *UDPConn) StartReadRequest(udpPutter proxy.UDP_Putter) {

	if len(u.clientSupposedAddr.IP) < 3 || u.clientSupposedAddr.IP.IsUnspecified() {
		u.clientSupposedAddrIsNothing = true
	}

	bs := make([]byte, proxy.MaxUDP_packetLen)
	for {
		n, addr, err := u.UDPConn.ReadFromUDP(bs)
		if err != nil {
			if utils.CanLogWarn() {
				log.Println("UDPConn read err", err)

			}
			continue
		}
		if n < 6 {
			if utils.CanLogWarn() {

				log.Println("UDPConn short read err", n)
			}
			continue
		}

		if !u.clientSupposedAddrIsNothing && !addr.IP.Equal(u.clientSupposedAddr.IP) || addr.Port != u.clientSupposedAddr.Port {

			//just random attack message.
			continue

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
			if utils.CanLogWarn() {

				log.Println("UDPConn unknown address type ", atyp)
			}
			continue

		}

		if len(bs[off:]) < l {
			if utils.CanLogWarn() {

				log.Println("UDPConn short command request ", atyp)
			}
			continue

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

		thisaddr := &netLayer.Addr{
			IP:    theIP,
			Name:  theName,
			Port:  thePort,
			IsUDP: true,
		}

		udpPutter.WriteUDPRequest(thisaddr.ToUDPAddr(), bs[newStart:n-newStart])

	}
}
