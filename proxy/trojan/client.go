package trojan

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//作为对照，可以参考 https://github.com/p4gefau1t/trojan-go/blob/master/tunnel/trojan/client.go

type ClientCreator struct{}

func (_ ClientCreator) NewClientFromURL(u *url.URL) (proxy.Client, error) {
	return nil, errors.New("not implemented")
}

func (_ ClientCreator) NewClient(dc *proxy.DialConf) (proxy.Client, error) {

	uuidStr := dc.Uuid

	c := Client{
		password_hexStringBytes: SHA224_hexStringBytes(uuidStr),
	}

	return &c, nil
}

type Client struct {
	proxy.ProxyCommonStruct
	password_hexStringBytes []byte
}

func (c *Client) Name() string {
	return name
}

func WriteTargetToBuf(target netLayer.Addr, buf *bytes.Buffer) {
	if len(target.IP) > 0 {
		if ip4 := target.IP.To4(); ip4 == nil {
			buf.WriteByte(netLayer.AtypIP6)
			buf.Write(target.IP)
		} else {
			buf.WriteByte(netLayer.AtypIP4)
			buf.Write(ip4)
		}
	} else if l := len(target.Name); l > 0 {
		buf.WriteByte(ATypDomain)
		buf.WriteByte(byte(l))
		buf.WriteString(target.Name)
	}

	buf.WriteByte(byte(target.Port >> 8))
	buf.WriteByte(byte(target.Port << 8 >> 8))
	buf.Write(crlf)
}

func (c *Client) Handshake(underlay net.Conn, target netLayer.Addr) (io.ReadWriteCloser, error) {
	if target.Port <= 0 {
		return nil, errors.New("Trojan Client Handshake failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.Write(c.password_hexStringBytes)
	buf.Write(crlf)
	buf.WriteByte(CmdConnect)
	WriteTargetToBuf(target, buf)

	_, err := underlay.Write(buf.Bytes())
	utils.PutBuf(buf)
	if err != nil {
		return nil, err
	}

	return underlay, nil
}

func (c *Client) EstablishUDPChannel(underlay net.Conn, target netLayer.Addr) (netLayer.MsgConn, error) {
	if target.Port <= 0 {
		return nil, errors.New("Trojan Client Handshake failed, target port invalid")

	}
	buf := utils.GetBuf()
	buf.Write(c.password_hexStringBytes)
	buf.Write(crlf)
	buf.WriteByte(CmdUDPAssociate)
	WriteTargetToBuf(target, buf)
	_, err := underlay.Write(buf.Bytes())
	utils.PutBuf(buf)
	if err != nil {
		return nil, err
	}

	return UDPConn{underlay}, nil
}
