// Package vless implements vless v0/v1 for proxy.Client and proxy.Server
package vless

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const Name = "vless"

const (
	addon_udp_multi_flag = 1 // for v1
)

// CMD types, for vless and vmess
const (
	_ byte = iota
	CmdTCP
	CmdUDP
	CmdMux
)

//依照 vless 协议的格式 依次读取 地址的 port, 域名/ip 信息
func GetAddrFrom(buf utils.ByteReader) (addr netLayer.Addr, err error) {

	pb1, err := buf.ReadByte()
	if err != nil {
		return
	}

	pb2, err := buf.ReadByte()
	if err != nil {
		return
	}

	port := uint16(pb1)<<8 + uint16(pb2)
	if port == 0 {
		err = utils.ErrInvalidData
		return
	}
	addr.Port = int(port)

	var b1 byte

	b1, err = buf.ReadByte()
	if err != nil {
		return
	}

	switch b1 {
	case netLayer.AtypDomain:
		var b2 byte
		b2, err = buf.ReadByte()
		if err != nil {
			return
		}

		if b2 == 0 {
			err = errors.New("got ATypDomain but domain lenth is marked to be 0")
			return
		}

		bs := utils.GetBytes(int(b2))
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}

		if n != int(b2) {
			err = utils.ErrShortRead
			return
		}
		addr.Name = string(bs[:n])

	case netLayer.AtypIP4:
		bs := make([]byte, 4)
		var n int
		n, err = buf.Read(bs)

		if err != nil {
			return
		}
		if n != 4 {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	case netLayer.AtypIP6:
		bs := make([]byte, net.IPv6len)
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}
		if n != 4 {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	default:
		err = utils.ErrInvalidData
		return
	}

	return
}

//依照 vless 协议的格式 依次写入 地址的 port, 域名/ip 信息
func WriteAddrTo(writeBuf utils.ByteWriter, raddr netLayer.Addr) {
	writeBuf.WriteByte(byte(raddr.Port >> 8))
	writeBuf.WriteByte(byte(raddr.Port << 8 >> 8))
	abs, atyp := raddr.AddressBytes()
	writeBuf.WriteByte(atyp)
	writeBuf.Write(abs)

}

//https://github.com/XTLS/Xray-core/discussions/716
func GenerateXrayShareURL(dialconf *proxy.DialConf) string {

	var u url.URL

	u.Scheme = Name
	u.User = url.User(dialconf.Uuid)
	if dialconf.IP != "" {
		u.Host = dialconf.IP + ":" + strconv.Itoa(dialconf.Port)
	} else {
		u.Host = dialconf.Host + ":" + strconv.Itoa(dialconf.Port)

	}
	q := u.Query()
	if dialconf.TLS {
		q.Add("security", "tls")
		if dialconf.Host != "" {
			q.Add("sni", dialconf.Host)

		}
		if len(dialconf.Alpn) > 0 {
			var sb strings.Builder
			for i, s := range dialconf.Alpn {
				sb.WriteString(s)
				if i != len(dialconf.Alpn)-1 {
					sb.WriteString(",")
				}
			}
			q.Add("alpn", sb.String())

		}

		//if dialconf.Insecure{
		//	q.Add("allowInsecure", true)

		//}
	}
	if dialconf.AdvancedLayer != "" {
		q.Add("type", dialconf.AdvancedLayer)

		switch dialconf.AdvancedLayer {
		case "ws":
			if dialconf.Path != "" {
				q.Add("path", dialconf.Path)
			}
			if dialconf.Host != "" {
				q.Add("host", dialconf.Host)

			}
		case "grpc":
			if dialconf.Path != "" {
				q.Add("serviceName", dialconf.Path)
			}
		}
	}

	u.RawQuery = q.Encode()
	if dialconf.Tag != "" {
		u.Fragment = dialconf.Tag

	}

	return u.String()
}
