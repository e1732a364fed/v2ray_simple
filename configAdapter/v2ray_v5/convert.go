package v2ray_v5

import "github.com/e1732a364fed/v2ray_simple/proxy"

func ToVS(c *Conf) (s proxy.StandardConf, err error) {

	return
}

func FromVS(lc *proxy.StandardConf) (c Conf, err error) {
	if dc := lc.DnsConf; dc != nil {
		d := &DNSObject{}
		c.DNS = d
		switch dc.Strategy { //vs 的Strategy功能比v2的强大, 多了40和60 两个选项
		default:
			d.QS = "UseIP"
		case 4, 40:
			d.QS = "UseIPv4"
		case 6, 60:
			d.QS = "UseIPv6"
		}
		//v2ray 没有TTL strategy

	}
	return
}
