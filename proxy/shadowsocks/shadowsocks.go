/*
Package shadowsocks implements shadowsocks protocol.

Reference

https://github.com/shadowsocks/shadowsocks-org/wiki/Protocol

https://github.com/shadowsocks/shadowsocks-org/wiki/AEAD-Ciphers

这里vs参考了gost的实现。gost中，Connector就相当于 client，Handler就相当于 Server

但是发现，没法一个server同时处理tcp和udp？ 也就是说，只能预先指定服务端要处理的协议；

看ss的标准，也没有提及哪一项 可以指定 tcp/udp

重新阅读上面Protocol页面，参考阅读 http://overtalk.site/2020/02/25/network-shadowsocks/

ss不像vmess等协议一样，只使用一种传输层协议来传输 tcp和udp数据；而是：用tcp传tcp，用udp传udp。
如此的话，特征必很明显。

还有一个重要的问题，就是，我们vs的架构，在设计之初，就是为vmess/vless/trojan等 只需要一种传输层协议 来获取 多种传输层协议的客户端等数据的；

而为了支持ss，以目前的vs架构来说，要同时写两个listen，一个监听tcp，一个监听udp，如此才能做到。

而且对于client来说也比较棘手，因为我们的架构只认为需要dial单一的传输层协议就可以与一个服务端完整通信，所以配置文件里需要配置network指明使用的是哪个传输层协议；而如果是ss的模式的话，则客户端对tcp和udp都要拨号，也十分麻烦。



另外，本包是普通的ss AEAD Ciphers ，不过它还是有问题。所以以后要研究ss-2022

https://github.com/shadowsocks/shadowsocks-org/issues/183

关于ss-2022
https://github.com/shadowsocks/shadowsocks-org/issues/196

*/
package shadowsocks

import (
	"errors"
	"net"
	"net/url"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"go.uber.org/zap"
)

const Name = "shadowsocks"
const (
	ATypIP4    = 0x1
	ATypDomain = 0x3
	ATypIP6    = 0x4
)

func initShadowCipher(info MethodPass) (cipher core.Cipher) {
	var method, password = info.Method, info.Password
	//根据 https://github.com/shadowsocks/shadowsocks-org/wiki/SIP002-URI-Scheme

	if method == "" || password == "" {
		return
	}

	var err error
	cipher, err = core.PickCipher(strings.ToUpper(method), nil, password)
	if err != nil {
		if ce := utils.CanLogErr("ss initShadowCipher err"); ce != nil {
			ce.Write(zap.Error(err))
		}

		return
	}

	return
}

//依照shadowsocks协议的格式读取 地址的域名、ip、port信息 (same as socks5 and trojan)
func GetAddrFrom(buf utils.ByteReader) (addr netLayer.Addr, err error) {
	var b1 byte

	b1, err = buf.ReadByte()
	if err != nil {
		return
	}

	switch b1 {
	case ATypDomain:
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

	case ATypIP4:
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
	case ATypIP6:
		bs := make([]byte, net.IPv6len)
		var n int
		n, err = buf.Read(bs)
		if err != nil {
			return
		}
		if n != net.IPv6len {
			err = utils.ErrShortRead
			return
		}
		addr.IP = bs
	default:
		err = utils.ErrInErr{ErrDesc: "shadowsocks GetAddrFrom err", ErrDetail: utils.ErrInvalidData, Data: b1}
		return
	}

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
		err = utils.ErrInErr{ErrDesc: "shadowsocks GetAddrFrom, port is zero, which is bad", ErrDetail: utils.ErrInvalidData}
		return

	}
	addr.Port = int(port)

	return
}

type MethodPass struct {
	Method, Password string
}

//require "method" and "pass" field. return true if both not empty.
func (ph *MethodPass) InitWithUrl(u *url.URL) bool {
	ph.Method = u.Query().Get("method")
	ph.Password = u.Query().Get("pass")
	return len(ph.Method) > 0 && len(ph.Password) > 0
}

//uuid: "method:xxxx\npass:xxxx"
func (ph *MethodPass) InitWithStr(str string) (ok bool) {
	str = strings.TrimSuffix(str, "\n")
	strs := strings.SplitN(str, "\n", 2)
	if len(strs) != 2 {
		return
	}

	var potentialMethod, potentialPass string

	ustrs := strings.SplitN(strs[0], ":", 2)
	if ustrs[0] != "method" {

		return
	}
	potentialMethod = ustrs[1]

	pstrs := strings.SplitN(strs[1], ":", 2)
	if pstrs[0] != "pass" {

		return
	}
	potentialPass = pstrs[1]

	if potentialMethod != "" && potentialPass != "" {
		ph.Method = potentialMethod
		ph.Password = potentialPass
	}
	ok = true
	return
}
