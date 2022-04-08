package vless

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

//实现 proxy.UserClient，以及 netLayer.UDP_Putter
type Client struct {
	proxy.ProxyCommonStruct

	udpResponseChan chan netLayer.UDPAddrData

	version int

	user *proxy.V2rayUser

	is_CRUMFURS_established bool

	mutex                sync.RWMutex
	knownUDPDestinations map[string]io.ReadWriter

	crumfursBuf *bufio.Reader //在不使用ws或者grpc时，需要一个缓存 来读取 CRUMFURS
}

type ClientCreator struct{}

func (_ ClientCreator) NewClientFromURL(u *url.URL) (proxy.Client, error) {
	return NewClientByURL(u)
}

func (_ ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	uuidStr := dc.Uuid
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}

	c := Client{
		user: id,
	}

	c.knownUDPDestinations = make(map[string]io.ReadWriter)
	c.udpResponseChan = make(chan netLayer.UDPAddrData, 20)

	v := dc.Version
	if v >= 0 {

		if v == 1 {
			c.version = 1
		}

	}

	return &c, nil
}

func NewClientByURL(url *url.URL) (proxy.Client, error) {
	uuidStr := url.User.Username()
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}

	c := Client{
		user: id,
	}
	c.knownUDPDestinations = make(map[string]io.ReadWriter)
	c.udpResponseChan = make(chan netLayer.UDPAddrData, 20)

	vStr := url.Query().Get("version")
	if vStr != "" {
		v, err := strconv.Atoi(vStr)
		if err == nil {
			if v == 1 {
				c.version = 1
			}
		}
	}

	return &c, nil
}

func (c *Client) Name() string { return Name }
func (c *Client) Version() int { return c.version }
func (c *Client) GetUser() proxy.User {
	return c.user
}

func (c *Client) Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error) {
	var err error

	port := target.Port
	addr, atyp := target.AddressBytes()

	buf := c.getBufWithCmd(CmdTCP)

	buf.WriteByte(byte(uint16(port) >> 8))
	buf.WriteByte(byte(uint16(port) << 8 >> 8))

	buf.WriteByte(atyp)
	buf.Write(addr)

	_, err = underlay.Write(buf.Bytes())

	utils.PutBuf(buf)

	if c.version == 0 {
		return &UserTCPConn{
			Conn:            underlay,
			uuid:            *c.user,
			version:         c.version,
			underlayIsBasic: netLayer.IsBasicConn(underlay),
		}, err
	} else {
		return underlay, nil
	}

}

func (c *Client) EstablishUDPChannel(underlay net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {
	var err error

	if c.version == 1 && !c.is_CRUMFURS_established {

		//log.Println("尝试拨号 Cmd_CRUMFURS 信道")

		//这段代码明显有问题，如果直接dial的话，那就是脱离tls的裸协议，所以这里以后需要处理一下
		UMFURS_conn, err := target.Dial()
		if err != nil {
			if ce := utils.CanLogErr("尝试拨号 Cmd_CRUMFURS 信道时发生错误"); ce != nil {

				//log.Println("尝试拨号 Cmd_CRUMFURS 信道时发生错误")
				ce.Write(zap.Error(err))
			}
			return nil, err
		}
		buf := c.getBufWithCmd(Cmd_CRUMFURS)

		UMFURS_conn.Write(buf.Bytes())

		utils.PutBuf(buf)

		bs := []byte{0}
		n, err := UMFURS_conn.Read(bs)
		if err != nil || n == 0 || bs[0] != CRUMFURS_ESTABLISHED {
			if ce := utils.CanLogErr("尝试读取 Cmd_CRUMFURS 信道返回值 时发生错误"); ce != nil {

				//log.Println("尝试读取 Cmd_CRUMFURS 信道返回值 时发生错误")
				ce.Write(zap.Error(err))
			}
			return nil, err
		}

		c.is_CRUMFURS_established = true

		// 循环监听 UMFURS 信息
		go c.handle_CRUMFURS(UMFURS_conn)

	}

	buf := c.getBufWithCmd(CmdUDP)
	port := target.Port

	buf.WriteByte(byte(uint16(port) >> 8))
	buf.WriteByte(byte(uint16(port) << 8 >> 8))
	addr, atyp := target.AddressBytes()

	buf.WriteByte(atyp)
	buf.Write(addr)

	_, err = underlay.Write(buf.Bytes())

	utils.PutBuf(buf)

	return &UDPConn{Conn: underlay, version: c.version, isClientEnd: true, raddr: target}, err
}

func (c *Client) getBufWithCmd(cmd byte) *bytes.Buffer {
	v := c.version
	buf := utils.GetBuf()
	buf.WriteByte(byte(v)) //version
	buf.Write(c.user[:])
	if v == 0 {
		buf.WriteByte(0) //addon length
	}
	buf.WriteByte(cmd) // cmd
	return buf
}

// 把在 CRUMFURS 信道中 获取到的 未知流量 转发到 UDPResponseWriter （本作中就是 转发到 inServer 中, 而且 只有 socks5 这一种 inServer 实现了该方法， 见 main.go)
func (c *Client) handle_CRUMFURS(UMFURS_conn net.Conn) {

	if c.udpResponseChan == nil {
		c.is_CRUMFURS_established = false
		return
	}

	for {
		//之前讨论了，udp信息通通要传长度头，CRUMFURS 也不例外，在 没有AdvancedLayer时，统一都要加udp长度头

		if c.AdvancedL != "" {

			buf_for_umfurs := utils.GetPacket()
			n, err := UMFURS_conn.Read(buf_for_umfurs)
			if err != nil {
				break
			}
			if n < 7 {

				break
			}
			msg := buf_for_umfurs[:n]
			atyp := msg[0]
			portIndex := net.IPv4len
			switch atyp {
			case netLayer.AtypIP6:
				portIndex = net.IPv6len
			default:
				//不合法，必须是ipv4或者ipv6
				break

			}
			theIP := make(net.IP, portIndex)
			copy(theIP, msg[1:portIndex])

			port := int16(msg[portIndex])<<8 + int16(msg[portIndex+1])

			c.udpResponseChan <- netLayer.UDPAddrData{
				Addr: net.UDPAddr{
					IP:   theIP,
					Port: int(port),
				},
				Data: msg[portIndex+2:],
			}
		} else {
			if c.crumfursBuf == nil {
				c.crumfursBuf = bufio.NewReader(UMFURS_conn)
			}

			atyp, err := c.crumfursBuf.ReadByte()
			if err != nil {
				break
			}

			ipLen := net.IPv4len
			switch atyp {
			case netLayer.AtypIP6:
				ipLen = net.IPv6len
			default:
				//不合法，必须是ipv4或者ipv6
				break
			}

			theIP := make(net.IP, ipLen)
			_, err = c.crumfursBuf.Read(theIP)
			if err != nil {
				break
			}

			twoBytes, err := c.crumfursBuf.Peek(2)
			if err != nil {
				break
			}

			port := int(int16(twoBytes[0])<<8 + int16(twoBytes[1]))

			c.crumfursBuf.Discard(2)

			twoBytes, err = c.crumfursBuf.Peek(2)
			if err != nil {
				break
			}

			packetLen := int16(twoBytes[0])<<8 + int16(twoBytes[1])
			c.crumfursBuf.Discard(2)

			msg := make([]byte, packetLen)

			_, err = io.ReadFull(c.crumfursBuf, msg)
			if err != nil {
				break
			}

			c.udpResponseChan <- netLayer.UDPAddrData{
				Addr: net.UDPAddr{
					IP:   theIP,
					Port: port,
				},
				Data: msg,
			}

		}
	}

	c.is_CRUMFURS_established = false
}
