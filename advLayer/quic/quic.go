//Package quic defines functions to listen and dial quic, with some customizable congestion settings.
//
// 这里我们 还选择性 使用 hysteria的 brutal阻控.
// 见 https://github.com/tobyxdd/quic-go 中 toby的 *-mod 分支, 里面会多一个 congestion 文件夹.
package quic

import (
	"log"
	"reflect"
	"time"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/lucas-clemente/quic-go"
)

func init() {
	advLayer.ProtocolsMap["quic"] = Creator{}
}

//quic的包装太简单了

//超简单，直接参考 https://github.com/lucas-clemente/quic-go/blob/master/example/echo/echo.go

//我们这里利用了hysteria的阻控，但是没有使用hysteria的通知速率和 auth的 数据头，也就是说我们这里是纯quic协议的情况下使用了hysteria的优点。

//但是我在mac里实测，内网单机极速测速的情况下，本来tcp能达到3000mbps的速度，到了quic就只能达到 1333mbps左右。

//我们要是以后不使用hysteria的话，只需删掉 useHysteria 里的代码, 删掉 pacer.go/brutal.go, 并删掉 go.mod中的replace部分.
// 然后proxy.go里的 相关配置部分也要删掉 在 prepareTLS_for* 函数中 的相关配置 即可.

const (
	//100mbps
	Default_hysteriaMaxByteCount = 1024 * 1024 / 8 * 100

	common_maxidletimeout          = time.Second * 45
	common_HandshakeIdleTimeout    = time.Second * 8
	common_ConnectionIDLength      = 12
	server_maxStreamCountInOneConn = 4 //一个 Connection 中 stream越多, 性能越低, 因此我们这里限制为4
)

func isActive(s quic.Connection) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func CloseConn(baseC any) {
	qc, ok := baseC.(quic.Connection)
	if ok {
		qc.CloseWithError(0, "")
	} else {
		log.Panicln("quic.CloseConn called with illegal parameter", reflect.TypeOf(baseC).String(), baseC)
	}
}

var (
	//h3
	DefaultAlpnList = []string{"h3"}

	common_ListenConfig = quic.Config{
		ConnectionIDLength:    common_ConnectionIDLength,
		HandshakeIdleTimeout:  common_HandshakeIdleTimeout,
		MaxIdleTimeout:        common_maxidletimeout,
		MaxIncomingStreams:    server_maxStreamCountInOneConn,
		MaxIncomingUniStreams: -1,
		KeepAlive:             true,
	}

	common_DialConfig = quic.Config{
		ConnectionIDLength:   common_ConnectionIDLength,
		HandshakeIdleTimeout: common_HandshakeIdleTimeout,
		MaxIdleTimeout:       common_maxidletimeout,
		KeepAlive:            true,
	}
)

type Creator struct{}

func (Creator) GetDefaultAlpn() (alpn string, mustUse bool) {
	return "h3", false
}

func (Creator) PackageID() string {
	return "quic"
}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {
	var alpn []string
	if conf.TlsConf != nil {
		alpn = conf.TlsConf.NextProtos

	}

	var useHysteria, hysteria_manual bool
	var maxbyteCount int

	if len(alpn) == 0 {
		alpn = DefaultAlpnList
	}

	if conf.Extra != nil {
		useHysteria, hysteria_manual, maxbyteCount, _ = getExtra(conf.Extra)
	}

	return NewClient(&conf.Addr, alpn, conf.Host, conf.TlsConf.InsecureSkipVerify, useHysteria, maxbyteCount, hysteria_manual, conf.IsEarly), nil
}

func (Creator) NewServerFromConf(conf *advLayer.Conf) (advLayer.Server, error) {

	var useHysteria, hysteria_manual bool
	var maxbyteCount int
	var maxStreamCountInOneSession int64

	tlsConf := *conf.TlsConf
	if len(tlsConf.NextProtos) == 0 {
		tlsConf.NextProtos = DefaultAlpnList
	}

	if conf.Extra != nil {

		useHysteria, hysteria_manual, maxbyteCount, maxStreamCountInOneSession = getExtra(conf.Extra)

	}

	return &Server{
		addr:                          conf.Addr.String(),
		tlsConf:                       tlsConf,
		useHysteria:                   useHysteria,
		hysteria_manual:               hysteria_manual,
		hysteriaMaxByteCount:          maxbyteCount,
		customMaxStreamCountInOneConn: maxStreamCountInOneSession,
	}, nil
}

func getExtra(extra map[string]any) (useHysteria, hysteria_manual bool,
	maxbyteCount int,
	maxStreamCountInOneSession int64) {

	if thing := extra["maxStreamCountInOneSession"]; thing != nil {
		if count, ok := thing.(int64); ok && count > 0 {
			log.Println("maxStreamCountInOneSession,", count)
			maxStreamCountInOneSession = count

		}

	}

	if thing := extra["congestion_control"]; thing != nil {
		if use, ok := thing.(string); ok && use == "hy" {
			useHysteria = true

			if thing := extra["mbps"]; thing != nil {
				if mbps, ok := thing.(int64); ok && mbps > 1 {
					maxbyteCount = int(mbps) * 1024 * 1024 / 8

					log.Println("Using Hysteria Congestion Control, max upload mbps: ", mbps)

				}
			} else {

				log.Println("Using Hysteria Congestion Control, max upload mbps:", Default_hysteriaMaxByteCount, "mbps")

			}

			if thing := extra["hy_manual"]; thing != nil {
				if ismanual, ok := thing.(bool); ok {
					hysteria_manual = ismanual
					if ismanual {
						log.Println("Using Hysteria Manual Control Mode")
					}
				}
			}
		}
	}

	return
}
