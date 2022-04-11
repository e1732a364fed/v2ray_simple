package vless

import (
	"bufio"
	"io"
	"net"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func (s *Server) Get_CRUMFURS(id string) *CRUMFURS {
	bs, err := utils.StrToUUID(id)
	if err != nil {
		return nil
	}
	return s.userCRUMFURS[bs]
}

type CRUMFURS struct {
	net.Conn
	hasAdvancedLayer bool //在用ws或grpc时，这个开关保持打开
}

func (c *CRUMFURS) WriteUDPResponse(a net.UDPAddr, b []byte) (err error) {
	atype := netLayer.AtypIP4
	if len(a.IP) > 4 {
		atype = netLayer.AtypIP6
	}
	buf := utils.GetBuf()

	buf.WriteByte(atype)
	buf.Write(a.IP)
	buf.WriteByte(byte(int16(a.Port) >> 8))
	buf.WriteByte(byte(int16(a.Port) << 8 >> 8))

	if !c.hasAdvancedLayer {
		lb := int16(len(b))

		buf.WriteByte(byte(lb >> 8))
		buf.WriteByte(byte(lb << 8 >> 8))
	}
	buf.Write(b)

	_, err = c.Write(buf.Bytes())

	utils.PutBuf(buf)
	return
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
