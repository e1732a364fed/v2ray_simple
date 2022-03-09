package vless

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

func init() {
	proxy.RegisterClient(Name, NewVlessClient)
}

type Client struct {
	proxy.ProxyCommonStruct

	udpResponseChan chan *proxy.UDPAddrData

	version int

	user *proxy.ID

	is_CRUMFURS_established bool

	mutex                sync.RWMutex
	knownUDPDestinations map[string]io.ReadWriter
}

func NewVlessClient(url *url.URL) (proxy.Client, error) {
	addr := url.Host
	uuidStr := url.User.Username()
	id, err := proxy.NewID(uuidStr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: addr},
		user:              id,
	}
	c.ProxyCommonStruct.InitFromUrl(url)

	vStr := url.Query().Get("version")
	if vStr != "" {
		v, err := strconv.Atoi(vStr)
		if err == nil {
			if v == 1 {
				c.version = 1
				c.knownUDPDestinations = make(map[string]io.ReadWriter)
				c.udpResponseChan = make(chan *proxy.UDPAddrData, 20)
			}
		}
	}

	return c, nil
}

func (c *Client) Name() string { return Name }
func (c *Client) Version() int { return c.version }

func (c *Client) Handshake(underlay net.Conn, target *proxy.Addr) (io.ReadWriter, error) {
	var err error

	if underlay == nil {
		underlay, err = target.Dial()
		if err != nil {
			return nil, err
		}
	}

	port := target.Port
	addr, atyp := target.AddressBytes()

	cmd := proxy.CmdTCP
	if target.IsUDP {
		if c.version == 1 && !c.is_CRUMFURS_established {

			UMFURS_conn, err := target.Dial()
			if err != nil {
				log.Println("尝试拨号 Cmd_CRUMFURS 信道时发生错误")
				return nil, err
			}
			buf := c.getBufWithCmd(Cmd_CRUMFURS)

			UMFURS_conn.Write(buf.Bytes())

			bs := []byte{0}
			n, err := UMFURS_conn.Read(bs)
			if err != nil || n == 0 || bs[0] != CRUMFURS_ESTABLISHED {
				log.Println("尝试读取 Cmd_CRUMFURS 信道返回值 时发生错误")
				return nil, err
			}

			c.is_CRUMFURS_established = true

			// 循环监听 UMFURS 信息
			go c.handle_CRUMFURS(UMFURS_conn)

		}

		cmd = proxy.CmdUDP

	}

	buf := c.getBufWithCmd(cmd)
	err = binary.Write(buf, binary.BigEndian, uint16(port)) // port
	if err != nil {
		return nil, err
	}
	buf.WriteByte(atyp)
	buf.Write(addr)

	_, err = underlay.Write(buf.Bytes())

	return &UserConn{
		Conn:    underlay,
		uuid:    c.user.UUID,
		version: c.version,
		isUDP:   target.IsUDP,
	}, err
}

func (c *Client) GetNewUDPResponse() (*net.UDPAddr, []byte, error) {
	x := <-c.udpResponseChan //由 handle_CRUMFURS 以及 WriteUDPRequest 中的 goroutine 填充
	return x.Addr, x.Data, nil
}

func (c *Client) WriteUDPRequest(a *net.UDPAddr, b []byte) (err error) {

	astr := a.String()

	c.mutex.RLock()
	knownConn := c.knownUDPDestinations[astr]
	c.mutex.RUnlock()

	if knownConn == nil {

		knownConn, err = c.Handshake(nil, proxy.NewAddrFromUDPAddr(a))
		if err != nil {
			return err
		}

		c.mutex.Lock()
		c.knownUDPDestinations[astr] = knownConn
		c.mutex.Unlock()

		go func() {
			bs := make([]byte, proxy.MaxUDP_packetLen)
			for {
				n, err := knownConn.Read(bs)
				if err != nil {
					break
				}
				if n <= 0 {
					continue
				}
				msg := make([]byte, n)
				copy(msg, bs[:n])

				c.udpResponseChan <- &proxy.UDPAddrData{
					Addr: a,
					Data: msg,
				}
			}
		}()
	}

	_, err = knownConn.Write(b)
	return
}

func (c *Client) getBufWithCmd(cmd byte) *bytes.Buffer {
	v := c.version
	buf := &bytes.Buffer{}
	buf.WriteByte(byte(v)) //version
	buf.Write(c.user.UUID[:])
	if v == 0 {
		buf.WriteByte(0) //addon length
	}
	buf.WriteByte(cmd) // cmd
	return buf
}

// 把在 CRUMFURS 信道中 获取到的 未知流量 转发到 UDPResponseWriter （本v2simple中就是 转发到localServer中, 而且 只有 socks5 这一种localServer实现了该方法， 见 main.go)
func (c *Client) handle_CRUMFURS(UMFURS_conn net.Conn) {

	if c.udpResponseChan == nil {
		c.is_CRUMFURS_established = false
		return
	}

	for {
		buf_for_umfurs := make([]byte, proxy.MaxUDP_packetLen)
		n, err := UMFURS_conn.Read(buf_for_umfurs)
		if err != nil {
			break
		}
		if n < 7 {
			// 信息长度至少要为7才行。
			break
		}
		msg := buf_for_umfurs[:n]
		atyp := msg[0]
		portIndex := net.IPv4len
		switch atyp {
		case proxy.AtypIP4:

		case proxy.AtypIP6:
			portIndex = net.IPv6len
		default:
			//不合法，必须是ipv4或者ipv6
			break

		}
		theIP := make(net.IP, portIndex)
		copy(theIP, msg[1:portIndex])

		port := int16(msg[portIndex])<<8 + int16(msg[portIndex+1])

		c.udpResponseChan <- &proxy.UDPAddrData{
			Addr: &net.UDPAddr{
				IP:   theIP,
				Port: int(port),
			},
			Data: msg[portIndex+2:],
		}

	}

	c.is_CRUMFURS_established = false
}
