/*Package quic defines functions to listen and dial quic, with some customizable congestion settings.

quic的包装太简单了, 直接参考 https://github.com/lucas-clemente/quic-go/blob/master/example/echo/echo.go

Hysteria Brutal Congestion Control

这里我们 还选择性 使用 hysteria的 brutal阻控.
见 https://github.com/tobyxdd/quic-go 中 toby的 *-mod 分支, 里面会多一个 congestion 文件夹.

我们这里利用其阻控，但是没有使用hysteria的通知速率和 auth的 数据头，也就是说我们这里是纯quic协议的情况下使用了hysteria的优点。

我们要是以后不使用hysteria的话，只需删掉 useHysteria 里的代码, 删掉 pacer.go/brutal.go, 并删掉 go.mod中的replace部分.
然后proxy.go里的 相关配置部分也要删掉 在 prepareTLS_for* 函数中 的相关配置 即可.


Imperfect Package

引用的 lucas-clemente/quic-go 包 是不完美的，它对阻塞控制支持不好，而且cpu占用率高。以后有更好的包的话要及时切换到好包。

我在macm1里实测，内网单机极速测速的情况下，本来tcp的vlesss能达到3000mbps的速度，到了vless+quic就只能达到 1333mbps左右。

实际上就是因为quic-go包占用cpu太猛了，导致cpu占用率达到100%瓶颈，所以速度无法进一步提高。

*/
package quic

import (
	"crypto/tls"
	"log"
	"reflect"
	"time"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/lucas-clemente/quic-go"
	"go.uber.org/zap"
)

func init() {
	advLayer.ProtocolsMap["quic"] = Creator{}
}

const (
	//100mbps
	Default_hysteriaMaxByteCount           = 1024 * 1024 / 8 * 100
	Default_Server_maxStreamCountInOneConn = 4 //一个 Connection 中 stream越多, 性能越低, 因此我们这里限制为4

	common_maxidletimeout       = time.Second * 45
	common_HandshakeIdleTimeout = time.Second * 8
	common_ConnectionIDLength   = 12
)

var (
	//h3
	DefaultAlpnList = []string{"h3"}

	common_ListenConfig = quic.Config{
		ConnectionIDLength:    common_ConnectionIDLength,
		HandshakeIdleTimeout:  common_HandshakeIdleTimeout,
		MaxIdleTimeout:        common_maxidletimeout,
		MaxIncomingStreams:    Default_Server_maxStreamCountInOneConn,
		MaxIncomingUniStreams: -1,
		KeepAlivePeriod:       time.Minute,
	}

	common_DialConfig = quic.Config{
		ConnectionIDLength:   common_ConnectionIDLength,
		HandshakeIdleTimeout: common_HandshakeIdleTimeout,
		MaxIdleTimeout:       common_maxidletimeout,
		KeepAlivePeriod:      time.Minute,
	}
)

func isActive(s quic.Connection) bool {
	select {
	case <-s.Context().Done():
		return false
	default:
		return true
	}
}

func CloseConn(conn any) {
	qc, ok := conn.(quic.Connection)
	if ok && qc != nil {
		qc.CloseWithError(0, "")
	} else {
		if ce := utils.CanLogErr("Quic.CloseConn called with illegal parameter"); ce != nil {
			ce.Write(zap.String("type", reflect.TypeOf(conn).String()), zap.Any("value", conn))
		}

	}
}

type arguments struct {
	useHysteria, hysteria_manual, early bool
	customMaxStreamsInOneConn           int64
	hysteriaMaxByteCount                int
}

type Creator struct{}

func (Creator) GetDefaultAlpn() (alpn string, mustUse bool) {
	return "h3", false
}

func (Creator) PackageID() string {
	return "quic"
}

func (Creator) ProtocolName() string {
	return "quic"
}

func (Creator) CanHandleHeaders() bool {
	return false
}

func (Creator) IsSuper() bool {
	return true
}

func (Creator) IsMux() bool {
	return true
}

func (Creator) NewClientFromConf(conf *advLayer.Conf) (advLayer.Client, error) {
	var alpn []string
	if conf.TlsConf != nil {
		alpn = conf.TlsConf.NextProtos
	}
	if len(alpn) == 0 {
		alpn = DefaultAlpnList
	}

	var useHysteria, hysteria_manual bool
	var maxbyteCount int

	if conf.Extra != nil {
		useHysteria, hysteria_manual, maxbyteCount, _ = getExtra(conf.Extra)
	}

	var tConf tls.Config
	if conf.TlsConf != nil {
		tConf = *conf.TlsConf

	}
	tConf.NextProtos = alpn

	return NewClient(&conf.Addr, tConf, arguments{
		early:                conf.IsEarly,
		useHysteria:          useHysteria,
		hysteria_manual:      hysteria_manual,
		hysteriaMaxByteCount: maxbyteCount,
	}), nil
}

func (Creator) NewServerFromConf(conf *advLayer.Conf) (advLayer.Server, error) {

	var useHysteria, hysteria_manual bool
	var maxbyteCount int
	var maxStreamCountInOneConn int64

	tlsConf := *conf.TlsConf
	if len(tlsConf.NextProtos) == 0 {
		tlsConf.NextProtos = DefaultAlpnList
	}

	if conf.Extra != nil {

		useHysteria, hysteria_manual, maxbyteCount, maxStreamCountInOneConn = getExtra(conf.Extra)

	}

	return &Server{
		addr:    conf.Addr.String(),
		tlsConf: tlsConf,
		args: arguments{
			useHysteria:               useHysteria,
			hysteria_manual:           hysteria_manual,
			hysteriaMaxByteCount:      maxbyteCount,
			customMaxStreamsInOneConn: maxStreamCountInOneConn,
			early:                     conf.IsEarly,
		},
	}, nil
}

//从配置map中读取更多配置信息。一般在程序刚运行时调用。为了保证信息绝对能输出，就是用了 log包来以防万一。
func getExtra(extra map[string]any) (useHysteria, hysteria_manual bool,
	maxbyteCount int,
	maxStreamsInOneConn int64) {

	if thing := extra["maxStreamsInOneConn"]; thing != nil {
		if count, ok := thing.(int64); ok && count > 0 {
			if ce := utils.CanLogInfo("Quic max Streams In One Conn"); ce != nil {
				ce.Write(zap.Int("count,", int(count)))
			} else {
				log.Println("Quic maxStreamsInOneConn,", count)

			}
			maxStreamsInOneConn = count

		}

	}

	if thing := extra["congestion_control"]; thing != nil {
		if use, ok := thing.(string); ok && use == "hy" {
			useHysteria = true

			if thing := extra["mbps"]; thing != nil {
				if mbps, ok := thing.(int64); ok && mbps > 1 {
					maxbyteCount = int(mbps) * 1024 * 1024 / 8
				}
			} else {
				maxbyteCount = Default_hysteriaMaxByteCount
			}

			if ce := utils.CanLogInfo("Using Hysteria Congestion Control"); ce != nil {
				ce.Write(zap.Int("max upload mbps,", int(maxbyteCount)))
			} else {
				log.Println("Using Hysteria Congestion Control, max upload mbps: ", maxbyteCount)

			}

			if thing := extra["hy_manual"]; thing != nil {
				if ismanual, ok := thing.(bool); ok {
					hysteria_manual = ismanual
					if ismanual {

						if ce := utils.CanLogInfo("Using Hysteria Manual Control Mode"); ce != nil {
							ce.Write()
						} else {
							log.Println("Using Hysteria Manual Control Mode")
						}

						if thing := extra["hy_manual_initial_rate"]; thing != nil {
							if initRate, ok := thing.(float64); ok {

								if rateOk(initRate) == 0 {
									TheCustomRate = initRate
									if ce := utils.CanLogInfo("Hysteria Manual Control Initial Rate"); ce != nil {
										ce.Write(zap.Float64("rate", initRate))
									} else {
										log.Println("Hysteria Manual Control Initial Rate", initRate)
									}

								} else {
									if ce := utils.CanLogErr("hy_manual_initial_rate not within threshold"); ce != nil {
										ce.Write(zap.Float64("wrongValue", initRate))
									} else {
										log.Println("hy_manual_initial_rate not within threshold", initRate)
									}

								}

							}
						}

					} // ismanual
				} // ismanual, ok
			} // hy_manual
		} // use hysteria
	} //congestion control

	return
}
