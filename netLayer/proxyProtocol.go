package netLayer

import (
	"io"
	"net"
	"strconv"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/pires/go-proxyproto"
)

var proxyProtocolListenPolicyFunc = func(upstream net.Addr) (proxyproto.Policy, error) { return proxyproto.REQUIRE, nil }

//PROXY protocol。
//Reference： http://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
//
//xver 必须是 1或者2, 本函数若遇到其他值会直接panic。wlc 为监听的连接，wrc为转发的连接。
// 另外，本函数只支持tcp。proxy protocol的 v2 是支持 udp的，但是本函数不支持udp。
func WritePROXYprotocol(xver int, wlc NetAddresser, wrc io.Writer) (n int, err error) {

	clientAddr, _ := NewAddrFromAny(wlc.RemoteAddr())
	selfAddr, _ := NewAddrFromAny(wlc.LocalAddr())

	buf := utils.GetBuf()

	switch xver {
	default:
		panic("Invalid Xver passed to netLayer.WritePROXYprotocol")
	case 1:

		headStr := "PROXY TCP4 "
		if clientAddr.IsIpv6() {
			headStr = "PROXY TCP6 "
		}

		buf.WriteString(headStr)
		buf.WriteString(clientAddr.IP.String())
		buf.WriteString(" ")
		buf.WriteString(selfAddr.IP.String())
		buf.WriteString(" ")
		buf.WriteString(strconv.Itoa(clientAddr.Port))
		buf.WriteString(" ")
		buf.WriteString(strconv.Itoa(selfAddr.Port))
		buf.WriteString("\r\n")

	case 2:
		buf.WriteString("\x0D\x0A\x0D\x0A\x00\x0D\x0A\x51\x55\x49\x54\x0A\x21") // signature + v2 + PROXY

		if clientAddr.IsIpv6() {

			buf.WriteString("\x21\x00\x24") // AF_INET6 + STREAM + 36 bytes

			buf.Write(clientAddr.IP.To16())
			buf.Write(selfAddr.IP.To16())

		} else {
			buf.WriteString("\x11\x00\x0C") // AF_INET + STREAM + 12 bytes
			buf.Write(clientAddr.IP.To4())
			buf.Write(selfAddr.IP.To4())

		}

		var p1 uint16 = uint16(clientAddr.Port)
		var p2 uint16 = uint16(selfAddr.Port)
		buf.Write([]byte{byte(p1 >> 8), byte(p1), byte(p2 >> 8), byte(p2)})

	}

	n, err = wrc.Write(buf.Bytes())
	utils.PutBuf(buf)

	return
}
