package vless

import (
	"bytes"
	"io"
	"net"
	"net/url"
	"strconv"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	proxy.RegisterClient(Name, ClientCreator{})
}

type ClientCreator struct{ proxy.CreatorCommonStruct }

func (ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	uuidStr := dc.UUID
	id, err := utils.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}

	c := Client{
		user: id,
	}

	v := dc.Version
	if v > 0 {

		if v == 1 {
			c.version = 1

			c.use_mux = dc.Mux

			if dc.Extra != nil {
				if thing := dc.Extra["vless1_udp_multi"]; thing != nil {
					if udp_multi, ok := thing.(bool); ok && udp_multi {
						c.udp_multi = true
					}
				}
			}
		} else {
			return nil, utils.ErrInErr{ErrDesc: "given version bigger than 1", ErrDetail: utils.ErrUnImplemented}
		}

	}

	return &c, nil
}

func (ClientCreator) URLToDialConf(url *url.URL, dc *proxy.DialConf, format int) (*proxy.DialConf, error) {
	switch format {
	case proxy.UrlStandardFormat:
		if dc == nil {
			dc = &proxy.DialConf{}
			uuidStr := url.User.Username()
			dc.UUID = uuidStr

		}

		return dc, nil
	default:
		return dc, utils.ErrUnImplemented

	}

}

// 实现 proxy.UserClient
type Client struct {
	proxy.Base

	version byte

	user utils.V2rayUser

	udp_multi bool
	use_mux   bool
}

func (*Client) GetCreator() proxy.ClientCreator {
	return ClientCreator{}
}
func (c *Client) Name() string {
	if c.version == 0 {
		return Name
	}
	return Name + "_" + strconv.Itoa(int(c.version))

	// 根据 https://forum.golangbridge.org/t/fmt-sprintf-vs-string-concatenation/23006
	// 直接 + 比 fmt.Sprintf 快不少.
}
func (c *Client) Version() int { return int(c.version) }
func (c *Client) GetUser() utils.User {
	return c.user
}

// 我们只支持 vless v1 的 mux
func (c *Client) HasInnerMux() (int, string) {
	if c.version == 1 && c.use_mux {
		return 2, "simplesocks"

	} else {
		return 0, ""

	}
}

func (c *Client) IsUDP_MultiChannel() bool {

	return c.udp_multi
}

func (c *Client) Handshake(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (io.ReadWriteCloser, error) {
	var err error

	port := target.Port
	addr, atyp := target.AddressBytes()

	var buf *bytes.Buffer
	if c.use_mux {
		buf = c.getBufWithCmd(CmdMux)

	} else {
		buf = c.getBufWithCmd(CmdTCP)
	}

	buf.WriteByte(byte(uint16(port) >> 8))
	buf.WriteByte(byte(uint16(port) << 8 >> 8))

	buf.WriteByte(atyp)
	buf.Write(addr)

	if len(firstPayload) > 0 {
		buf.Write(firstPayload)
		utils.PutBytes(firstPayload)
	}
	_, err = underlay.Write(buf.Bytes())

	utils.PutBuf(buf)

	if err != nil {
		return nil, err
	}

	if c.version == 0 {
		uc := &UserTCPConn{
			Conn:            underlay,
			V2rayUser:       c.user,
			version:         c.version,
			underlayIsBasic: netLayer.IsBasicConn(underlay),
		}
		if r, rr := netLayer.IsConnGoodForReadv(underlay); r != 0 {
			uc.rr = rr
			uc.readvType = r
			if r == 1 {
				uc.br = underlay.(utils.BuffersReader)
			}
		}
		if mw, ok := underlay.(utils.MultiWriter); ok {
			uc.mw = mw
		}

		return uc, nil
	} else {
		return underlay, nil
	}

}

func (c *Client) EstablishUDPChannel(underlay net.Conn, firstPayload []byte, target netLayer.Addr) (netLayer.MsgConn, error) {

	buf := c.getBufWithCmd(CmdUDP)
	port := target.Port

	buf.WriteByte(byte(uint16(port) >> 8))
	buf.WriteByte(byte(uint16(port) << 8 >> 8))
	addr, atyp := target.AddressBytes()

	buf.WriteByte(atyp)
	buf.Write(addr)

	target.Network = "udp"

	uc := &UDPConn{
		Conn:         underlay,
		V2rayUser:    c.user,
		version:      c.version,
		isClientEnd:  true,
		raddr:        target,
		udp_multi:    c.udp_multi,
		handshakeBuf: buf,
		fullcone:     c.IsFullcone,
	}
	if len(firstPayload) == 0 {
		return uc, nil
	} else {
		return uc, uc.WriteMsg(firstPayload, target)
	}

}

func (c *Client) getBufWithCmd(cmd byte) *bytes.Buffer {
	v := c.version
	buf := utils.GetBuf()
	buf.WriteByte(byte(c.version)) //version
	buf.Write(c.user[:])
	if v == 0 {
		buf.WriteByte(0) //addon length
	} else {
		switch {
		default:
			buf.WriteByte(0) //no addon
		case c.udp_multi:
			buf.WriteByte(addon_udp_multi_flag)
		}

	}
	buf.WriteByte(cmd) // cmd
	return buf
}
