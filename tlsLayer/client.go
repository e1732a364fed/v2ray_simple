package tlsLayer

import (
	"crypto/tls"
	"net"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	utls "github.com/refraction-networking/utls"
	"go.uber.org/zap"
)

// 关于utls的简单分析，可参考
//https://github.com/e1732a364fed/v2ray_simple/discussions/7

type Client struct {
	tlsConfig  *tls.Config
	uTlsConfig utls.Config
	tlsType    int
	alpnList   []string

	shadowTlsPassword string
}

func NewClient(conf Conf) *Client {

	c := &Client{
		tlsType: conf.Tls_type,
	}

	c.alpnList = conf.AlpnList

	switch conf.Tls_type {
	case ShadowTls2_t:
		fallthrough
	case ShadowTls_t:
		//tls和utls配置都设一遍，以备未来支持调节 shadowTls 所使用的 到底是utls还是tls
		c.tlsConfig = GetTlsConfig(false, conf)
		c.shadowTlsPassword = getShadowTlsPasswordFromExtra(conf.Extra)
		fallthrough
	case UTls_t:
		c.uTlsConfig = GetUTlsConfig(conf)

		if ce := utils.CanLogInfo("Using uTls and Chrome fingerprint for"); ce != nil {
			ce.Write(zap.String("host", conf.Host))
		}
	default:
		c.tlsConfig = GetTlsConfig(false, conf)

	}

	return c
}

func (c *Client) Handshake(underlay net.Conn) (tlsConn *Conn, err error) {

	switch c.tlsType {
	case UTls_t:
		configCopy := c.uTlsConfig //发现uTlsConfig竟然没法使用指针，握手一次后配置文件就会被污染，只能拷贝
		//否则的话接下来的握手客户端会报错： tls: CurvePreferences includes unsupported curve

		utlsConn := utls.UClient(underlay, &configCopy, utls.HelloChrome_Auto)
		err = utlsConn.Handshake()
		if err != nil {
			return
		}
		tlsConn = &Conn{
			Conn:    utlsConn,
			ptr:     unsafe.Pointer(utlsConn.Conn),
			tlsType: UTls_t,
		}
	case Tls_t:
		officialConn := tls.Client(underlay, c.tlsConfig)
		err = officialConn.Handshake()
		if err != nil {
			return
		}

		tlsConn = &Conn{
			Conn:    officialConn,
			ptr:     unsafe.Pointer(officialConn),
			tlsType: Tls_t,
		}
	case ShadowTls_t:

		err = tls.Client(underlay, c.tlsConfig).Handshake()
		if err != nil {
			return
		}

		tlsConn = &Conn{
			Conn:    underlay,
			tlsType: ShadowTls_t,
		}

	case ShadowTls2_t:

		hashR := utils.NewHashReader(underlay, []byte(c.shadowTlsPassword))

		rw := &netLayer.IOWrapper{
			Reader: hashR,
			Writer: underlay,
		}

		configCopy := c.uTlsConfig

		err = utls.UClient(rw, &configCopy, utls.HelloChrome_Auto).Handshake()
		if err != nil {
			return
		}

		// err = tls.Client(rw, c.tlsConfig).Handshake()
		// if err != nil {
		// 	return
		// }

		tlsConn = &Conn{
			Conn: &shadowClientConn{
				FakeAppDataConn: &FakeAppDataConn{Conn: rw},
				sum:             hashR.Sum(),
			},
			tlsType: ShadowTls2_t,
		}

	}

	return
}
