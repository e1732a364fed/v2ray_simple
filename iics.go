package v2ray_simple

import (
	"bytes"
	"net"
	"net/http"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

//一个贯穿转发流程的关键结构,简称iics
type incomingInserverConnState struct {

	// 在多路复用的情况下, 可能产生多个 IncomingInserverConnState，
	// 共用一个 baseLocalConn, 但是 wrappedConn 各不相同。
	// 所以一般我们不使用 指针传递 iics.

	baseLocalConn net.Conn     // baseLocalConn 是来自客户端的原始传输层链接
	wrappedConn   net.Conn     // wrappedConn 是层层握手后,代理层握手前 包装的链接,一般为tls层/高级层;
	inServer      proxy.Server //可为 nil
	defaultClient proxy.Client

	cachedRemoteAddr string
	theRequestPath   string

	inServerTlsConn            *tlsLayer.Conn
	inServerTlsRawReadRecorder *tlsLayer.Recorder

	isFallbackH2        bool
	fallbackH2Request   *http.Request
	fallbackFirstBuffer *bytes.Buffer

	fallbackXver int

	isTlsLazyServerEnd bool

	shouldCloseInSerBaseConnWhenFinish bool

	routedToDirect bool

	routingEnv *proxy.RoutingEnv //used in passToOutClient
}

// 在调用 passToOutClient前遇到err时调用, 若找出了buf，设置iics，并返回true
func (iics *incomingInserverConnState) extractFirstBufFromErr(err error) bool {
	if ce := utils.CanLogWarn("failed in inServer proxy handshake"); ce != nil {
		ce.Write(
			zap.String("handler", iics.inServer.AddrStr()),
			zap.Error(err),
		)
	}

	if !iics.inServer.CanFallback() {
		iics.wrappedConn.Close()
		return false
	}

	//通过err找出 并赋值给 iics.theFallbackFirstBuffer
	{

		fe, ok := err.(*utils.ErrBuffer)
		if !ok {
			// 能fallback 但是返回的 err却不是fallback err，证明遇到了更大问题，可能是底层read问题，所以也不用继续fallback了
			iics.wrappedConn.Close()
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
func checkfallback(iics incomingInserverConnState) (targetAddr netLayer.Addr, result int) {
	//先检查 mainFallback，如果mainFallback中各项都不满足 or根本没有 mainFallback 再检查 defaultFallback

	//一般情况下 iics.RoutingEnv 都会给出，但是 如果是 热加载、tproxy、go test、单独自定义 调用 ListenSer 不给出env 等情况的话， iics.RoutingEnv 都是空值
	if iics.routingEnv != nil {

		if mf := iics.routingEnv.MainFallback; mf != nil {

			var thisFallbackType byte

			theRequestPath := iics.theRequestPath

			if iics.fallbackFirstBuffer != nil && theRequestPath == "" {
				var failreason int

				_, _, theRequestPath, failreason = httpLayer.GetH1RequestMethod_and_PATH_from_Bytes(iics.fallbackFirstBuffer.Bytes(), false)

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
				//默认似乎默认tls不会给出alpn和sni项？获得的是空值,也许是因为我用了自签名+insecure,所以导致server并不会设置连接好后所协商的ServerName
				// 而alpn则也是正常的, 不设置肯定就是空值
				alpn := inServerTlsConn.GetAlpn()

				if alpn != "" {
					fallback_params = append(fallback_params, alpn)
					thisFallbackType |= httpLayer.Fallback_alpn

				}

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

				if ce := utils.CanLogDebug("Fallback check"); ce != nil {
					if fbResult != nil {
						ce.Write(
							zap.String("matched", fbResult.Addr.String()),
						)
					} else {
						ce.Write(
							zap.String("no match", ""),
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

	//默认回落, 每个listen配置 都可 有一个自己独享的默认回落

	if defaultFallbackAddr := iics.inServer.GetFallback(); defaultFallbackAddr != nil {

		targetAddr = *defaultFallbackAddr
		result = 0

	}
	return
}
