package v2ray_simple

import (
	"bytes"
	"math/rand"
	"net"
	"net/http"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var iicsZapWriterPool = sync.Pool{
	New: func() interface{} {
		return &iicsZapWriter{
			assignedFields: make([]zapcore.Field, 1, 4),
		}
	},
}

//专用于 iics的结构
type iicsZapWriter struct {
	ce             *zapcore.CheckedEntry
	assignedFields []zapcore.Field //始终保持 有且只有 一项
	id             uint32
}

func (zw *iicsZapWriter) setid(id uint32) {
	zw.assignedFields[0] = zap.Uint32("connid", id)
	zw.id = id
}

//只能调用Write一次，调用之后，zw 便不再可用。
func (zw *iicsZapWriter) Write(fields ...zapcore.Field) {
	if len(fields) > 0 {
		realFields := append(zw.assignedFields, fields...)

		zw.ce.Write(realFields...)

	} else {
		zw.ce.Write(zw.assignedFields...)

	}

	iicsZapWriterPool.Put(zw)
}

//一个贯穿转发流程的关键结构,简称iics
type incomingInserverConnState struct {
	id uint32 //6位数字(十进制), 用于标识每一个连接.

	// 在多路复用的情况下, 可能产生多个 IncomingInserverConnState，
	// 共用一个 baseLocalConn, 但是 wrappedConn 各不相同。
	// 所以一般我们不使用 指针传递 iics.

	baseLocalConn net.Conn     // baseLocalConn 是来自客户端的原始传输层链接
	wrappedConn   net.Conn     // wrappedConn 是层层握手后,代理层握手前 包装的链接,一般为tls层/高级层;
	inServer      proxy.Server //可为 nil
	defaultClient proxy.Client

	cachedRemoteAddr string

	inServerTlsConn            *tlsLayer.Conn
	inServerTlsRawReadRecorder *tlsLayer.Recorder

	isFallbackH2        bool
	fallbackRequestPath string
	fallbackH2Request   *http.Request
	fallbackFirstBuffer *bytes.Buffer

	fallbackXver int

	firstPayload   []byte
	udpFirstTarget netLayer.Addr

	isTlsLazyServerEnd bool //比如 listen 是 tls + vless 这种情况

	shouldCloseInSerBaseConnWhenFinish bool

	routedToDirect bool

	routingEnv *proxy.RoutingEnv //used in passToOutClient
}

//每个iics使用之前，必须调用 genID
func (iics *incomingInserverConnState) genID() {
	const low = 100000
	const hi = low*10 - 1
	iics.id = uint32(low + rand.Intn(hi-low))
}

// 在调用 passToOutClient前遇到err时调用, 若找出了buf，设置iics，并返回true
func (iics *incomingInserverConnState) extractFirstBufFromErr(err error) bool {
	if ce := iics.CanLogWarn("failed in inServer proxy handshake"); ce != nil {
		ce.Write(
			zap.String("handler", iics.inServer.AddrStr()),
			zap.Error(err),
		)
	}

	if !iics.inServer.CanFallback() {
		if iics.wrappedConn != nil {
			iics.wrappedConn.Close()

		}
		return false
	}

	//通过err找出 并赋值给 iics.theFallbackFirstBuffer
	{

		fe, ok := err.(*utils.ErrBuffer)
		if !ok {
			// 能fallback 但是返回的 err却不是fallback err，证明遇到了更大问题，可能是底层read问题，所以也不用继续fallback了
			if iics.wrappedConn != nil {
				iics.wrappedConn.Close()
			}
			return false
		}

		if firstbuffer := fe.Buf; firstbuffer == nil {
			//不应该，至少能读到1字节的。

			panic("No FirstBuffer")

		} else {
			iics.fallbackFirstBuffer = firstbuffer

		}
	}
	return true
}

//查看当前配置 是否支持fallback, 并获得回落地址。
// 被 passToOutClient 调用. 若 无fallback则 result < 0, 否则返回所使用的 PROXY protocol 版本, 0 表示 回落但是不用 PROXY protocol.
//
// 本方法不会修改 iics的任何内容.
func (iics *incomingInserverConnState) checkfallback() (targetAddr netLayer.Addr, result int) {
	//先检查 mainFallback，如果mainFallback中各项都不满足 or根本没有 mainFallback 再检查 defaultFallback

	//一般情况下 iics.RoutingEnv 都会给出，但是 如果是 热加载、tproxy、go test、单独自定义 调用 ListenSer 不给出env 等情况的话， iics.RoutingEnv 都是空值
	if iics.routingEnv != nil {

		if mf := iics.routingEnv.MainFallback; mf != nil {

			var thisFallbackType byte

			theRequestPath := iics.fallbackRequestPath

			if iics.fallbackFirstBuffer != nil && theRequestPath == "" {
				var failreason int

				_, _, theRequestPath, _, failreason = httpLayer.ParseH1Request(iics.fallbackFirstBuffer.Bytes(), false)

				if failreason != 0 {
					theRequestPath = ""
				}

			}

			fallback_params := make([]string, 0, 4)

			if theRequestPath != "" {
				fallback_params = append(fallback_params, theRequestPath)
				thisFallbackType |= httpLayer.Fallback_path
			}

			if inServerTlsConn := iics.inServerTlsConn; inServerTlsConn != nil {
				alpn := inServerTlsConn.GetAlpn()

				if alpn != "" {
					fallback_params = append(fallback_params, alpn)
					thisFallbackType |= httpLayer.Fallback_alpn

				}
				//默认似乎默认tls不会给出sni项？获得的是空值,也许是因为我用了自签名+insecure,所以导致server并不会设置连接好后所协商的ServerName

				sni := inServerTlsConn.GetSni()
				if sni != "" {
					fallback_params = append(fallback_params, sni)
					thisFallbackType |= httpLayer.Fallback_sni
				}
			}

			{
				fromTag := iics.inServer.GetTag()

				fbResult := mf.GetFallback(fromTag, thisFallbackType, fallback_params...)
				if fbResult == nil {
					fbResult = mf.GetFallback("", thisFallbackType, fallback_params...)
				}

				if ce := utils.CanLogDebug("Fallback to"); ce != nil {
					if fbResult != nil {
						ce.Write(
							zap.String("addr", fbResult.Addr.String()),
							zap.Any("params", fallback_params),
						)
					}
				}
				if fbResult != nil {
					targetAddr = fbResult.Addr
					result = fbResult.Xver
					return
				}
			}

		}

	}

	//默认回落, 每个listen配置 都 有一个自己独享的默认回落配置 (fallback = 80 这种)

	if defaultFallbackAddr := iics.inServer.GetFallback(); defaultFallbackAddr != nil {

		if ce := utils.CanLogDebug("Fallback to default setting"); ce != nil {
			ce.Write(
				zap.String("addr", defaultFallbackAddr.String()),
			)
		}
		targetAddr = *defaultFallbackAddr
		result = 0

	} else {

		result = -1
	}

	return
}

func (iics *incomingInserverConnState) CanLogInfo(msg string) *iicsZapWriter {
	return iics.CanLogLevel(utils.Log_info, msg)
}

func (iics *incomingInserverConnState) CanLogErr(msg string) *iicsZapWriter {
	return iics.CanLogLevel(utils.Log_error, msg)
}

func (iics *incomingInserverConnState) CanLogDebug(msg string) *iicsZapWriter {
	return iics.CanLogLevel(utils.Log_debug, msg)
}

func (iics *incomingInserverConnState) CanLogWarn(msg string) *iicsZapWriter {
	return iics.CanLogLevel(utils.Log_warning, msg)
}
func (iics *incomingInserverConnState) CanLogLevel(level int, msg string) *iicsZapWriter {
	if iics.id == 0 {
		iics.genID()
	}
	if ce := utils.CanLogLevel(level, msg); ce != nil {
		zw := iicsZapWriterPool.Get().(*iicsZapWriter)
		zw.ce = ce

		if zw.id != iics.id {
			zw.setid(iics.id)
		}

		return zw
	} else {
		return nil
	}
}
