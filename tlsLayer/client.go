package tlsLayer

import (
	"crypto/tls"
	"net"
	"strings"
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
	utlsFingerprint   utls.ClientHelloID
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

		if len(conf.Extra) > 0 {
			if thing := conf.Extra["utls_fingerprint"]; thing != nil {
				if str, ok := thing.(string); ok {
					str = strings.ToLower(str)
					switch str {
					case "chrome":
						fallthrough
					default:
						c.utlsFingerprint = utls.HelloChrome_Auto
					case "firefox":
						c.utlsFingerprint = utls.HelloFirefox_Auto

					case "ios":
						c.utlsFingerprint = utls.HelloIOS_Auto

					case "safari":
						c.utlsFingerprint = utls.HelloSafari_Auto

					case "golang":
						c.utlsFingerprint = utls.HelloGolang

					case "android":
						c.utlsFingerprint = utls.HelloAndroid_11_OkHttp

					case "360":
						c.utlsFingerprint = utls.Hello360_Auto

					case "edge":
						c.utlsFingerprint = utls.HelloEdge_Auto

					case "random":
						c.utlsFingerprint = utls.HelloRandomized

					}
				}
			}
		}

		if ce := utils.CanLogInfo("Using uTls and Chrome fingerprint for"); ce != nil {
			ce.Write(zap.String("host", conf.Host))
		}
	default:
		c.tlsConfig = GetTlsConfig(false, conf)

	}

	return c
}

// utls和tls时返回tlsLayer.Conn, shadowTls1时返回underlay, shadowTls2时返回 普通 net.Conn
func (c *Client) Handshake(underlay net.Conn) (result net.Conn, err error) {

	switch c.tlsType {
	case UTls_t:
		configCopy := c.uTlsConfig //发现uTlsConfig竟然没法使用指针，握手一次后配置文件就会被污染，只能拷贝
		//否则的话接下来的握手客户端会报错： tls: CurvePreferences includes unsupported curve

		if (c.utlsFingerprint == utls.ClientHelloID{}) {
			c.utlsFingerprint = utls.HelloChrome_Auto
		}

		utlsConn := utls.UClient(underlay, &configCopy, c.utlsFingerprint)
		err = utlsConn.Handshake()
		if err != nil {
			return
		}
		result = &conn{
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

		result = &conn{
			Conn:    officialConn,
			ptr:     unsafe.Pointer(officialConn),
			tlsType: Tls_t,
		}
	case ShadowTls_t:

		err = tls.Client(underlay, c.tlsConfig).Handshake()
		if err != nil {
			return
		}

		result = underlay

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

		result = &shadowClientConn{
			FakeAppDataConn: &FakeAppDataConn{Conn: rw},
			sum:             hashR.Sum(),
		}

	}

	return
}
