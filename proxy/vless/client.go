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

	//is_CRUMFURS_established bool

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

	//在vless v1中, 不使用 单独udp信道来传输所有raddr方向的数据
	// 所以在v1中，我们不应用 EstablishUDPChannel 函数
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
