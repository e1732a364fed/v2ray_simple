package ws

import (
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"net/http"

	"github.com/e1732a364fed/v2ray_simple/advLayer"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"go.uber.org/zap"
)

// 2048 /3 = 682.6666...  (682 又 三分之二),
// 683 * 4 = 2732, 若你不信，运行 we_test.go中的 TestBase64Len
const MaxEarlyDataLen_Base64 = 2732

var (
	connectionBs = []byte("Connection")
	upgradeBs    = []byte("Upgrade")
)

//implements advLayer.SingleServer
type Server struct {
	Creator
	UseEarlyData   bool
	Thepath        string
	RequestHeaders map[string][]string

	requestHeaderCheckCount     int
	noNeedToCheckRequestHeaders bool
	responseHeader              ws.HandshakeHeader
}

// 这里默认: 传入的path必须 以 "/" 为前缀. 本函数 不对此进行任何检查.
func NewServer(path string, headers *httpLayer.HeaderPreset, UseEarlyData bool) *Server {

	noNeedToCheckRequestHeaders := headers == nil || headers.Request == nil || len(headers.Request.Headers) == 0

	var requestHeaderCheckCount int

	var requestHeaders map[string][]string

	if !noNeedToCheckRequestHeaders {

		requestHeaders = headers.Request.Headers

		requestHeaderCheckCount = len(requestHeaders)

		//gobwas包首先过滤这些Header, 然后才是自定义的header，所以我们首先查看用户是否配置了这些header，
		// 如果有配置，则无视此header的配置。
		for k := range requestHeaders {
			switch k {
			case "Host", "Connection", "Upgrade", "Sec-WebSocket-Version", "Sec-WebSocket-Key", "Sec-WebSocket-Protocol", "Sec-WebSocket-Accept", "Sec-WebSocket-Extensions":
				requestHeaderCheckCount -= 1
				delete(requestHeaders, k)
			}
		}

	}

	var responseHeader ws.HandshakeHeader

	if headers != nil && headers.Response != nil && len(headers.Response.Headers) > 0 {
		responseHeader = ws.HandshakeHeaderHTTP(headers.Response.Headers)
	}

	return &Server{
		RequestHeaders:              requestHeaders,
		responseHeader:              responseHeader,
		noNeedToCheckRequestHeaders: noNeedToCheckRequestHeaders,
		requestHeaderCheckCount:     requestHeaderCheckCount,
		Thepath:                     path,
		UseEarlyData:                UseEarlyData,
	}
}

func (*Server) GetCreator() advLayer.Creator {
	return Creator{}
}
func (s *Server) GetPath() string {
	return s.Thepath
}

func (*Server) Stop() {}

// Handshake 用于 websocket的 Server 监听端，建立握手. 用到了 gobwas/ws.Upgrader.
//
// 返回可直接用于读写 websocket 二进制数据的 net.Conn. 如果遇到不符合的http1.1请求，会返回 httpLayer.FallbackMeta 和 httpLayer.ErrShouldFallback
func (s *Server) Handshake(underlay net.Conn) (net.Conn, error) {

	//我们目前只支持 ws on http1.1

	var rp httpLayer.H1RequestParser
	re := rp.ReadAndParse(underlay)
	if re != nil {
		if re == httpLayer.ErrNotHTTP_Request {
			if ce := utils.CanLogErr("WS check ErrNotHTTP_Request"); ce != nil {
				ce.Write()
			}

		} else {
			if ce := utils.CanLogErr("WS check handshake read failed"); ce != nil {
				ce.Write(zap.Error(re))
			}
		}
		return nil, utils.ErrInvalidData
	}

	optionalFirstBuffer := rp.WholeRequestBuf

	notWsRequest := false

	//因为 gobwas 会先自行给错误的连接 返回 错误信息，而这不行，所以我们先过滤一遍。
	//header 我们只过滤一个 connection 就行. 要是怕攻击者用 “对的path,method 和错误的header” 进行探测,
	// 那你设一个复杂的path就ok了。

	if rp.Method != "GET" || s.Thepath != rp.Path || len(rp.Headers) == 0 {
		notWsRequest = true

	} else {
		hasUpgrade := false
		for _, rh := range rp.Headers {
			httpLayer.CanonicalizeHeaderKey(rh.Head)
			if bytes.Equal(rh.Head, connectionBs) {

				httpLayer.CanonicalizeHeaderKey(rh.Value)
				if bytes.Equal(rh.Value, upgradeBs) {

					hasUpgrade = true
					break
				}
			}
		}
		if !hasUpgrade {
			notWsRequest = true
		}

	}

	if notWsRequest {
		return httpLayer.FallbackMeta{
			Conn:         underlay,
			H1RequestBuf: optionalFirstBuffer,
			Path:         rp.Path,
			Method:       rp.Method,
		}, httpLayer.ErrShouldFallback
	}

	var thePotentialEarlyData []byte

	requestHeaderNotGivenCount := s.requestHeaderCheckCount

	var theUpgrader *ws.Upgrader = &ws.Upgrader{

		//因为我们vs的架构，先统一监听tcp；然后再调用Handshake函数
		// 所以我们不能直接用http.Handle, 这也彰显了 用 gobwas/ws 包的好处
		// 给Upgrader提供的 OnRequest 专门用于过滤 path, 也不一定 需要我们的 httpLayer 去过滤

		// 我们的 httpLayer 的 过滤方法仍然是最安全的，可以杜绝 所有非法数据；
		// 而 ws.Upgrader.Upgrade 使用了 readLine 函数。如果客户提供一个非法的超长的一行的话，它就会陷入泥淖

		//我们这里就是先用 httpLayer 过滤 再和 buffer一起传入 ws 包
		// ReadBufferSize默认是 4096，已经够大

		OnHeader: func(key, value []byte) error {
			if s.noNeedToCheckRequestHeaders {
				return nil
			}
			vs := s.RequestHeaders[string(key)]
			if len(vs) > 0 {
				for _, v := range vs {
					if v == (string(value)) {
						requestHeaderNotGivenCount -= 1
						break
					}
				}
			}

			return nil
		},
		Header: s.responseHeader,
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			if requestHeaderNotGivenCount > 0 {
				if ce := utils.CanLogWarn("ws headers not match"); ce != nil {
					ce.Write(zap.Int("requestHeaderNotGivenCount", requestHeaderNotGivenCount))
				}
				return nil, ws.RejectConnectionError(ws.RejectionStatus(http.StatusBadRequest))
			}
			return s.responseHeader, nil
		},
	}

	if s.UseEarlyData {

		//xray和v2ray中，使用了 header中的
		// Sec-WebSocket-Protocol 字段 来传输 earlydata，来实现 0-rtt;我们为了兼容,同样用此字段
		// (websocket标准是没有定义 0-rtt的方法的，但是ws的握手包头部是可以自定义header的)

		// gobwas 的 Upgrader 用 ProtocolCustom 这个函数来检查 protocol的内容
		// 它会遍历客户端给出的所有 protocol，然后选择一个来返回

		//我们若提供了此函数，则必须返回true，否则 gobwas会返回 ErrMalformedRequest 错误
		theUpgrader.ProtocolCustom = func(b []byte) (string, bool) {
			//如果不提供custom方法的话，gobwas会使用 httphead.ScanTokens 来扫描所有的token
			// 其实就是扫描逗号分隔 的 字符串

			//但是因为我们是 earlydata，所以没有逗号，全部都是 base64 编码的内容, 所以直接读然后解码即可

			//还有要注意的是，因为这个是回调函数，所以需要是闭包 才能向我们实际连接储存数据，所以是无法直接放到通用的upgrader里的

			if len(b) > MaxEarlyDataLen_Base64 {
				return "", true
			}
			bs, err := base64.RawURLEncoding.DecodeString(string(b))
			if err != nil {
				// 传来的并不是base64数据，可能是其它访问我们网站websocket的情况，但是一般我们path复杂都会过滤掉，所以直接认为这是非法的
				return "", false
			}
			//if len(bs) != 0 && utils.CanLogDebug() {
			//	log.Println("Got New ws earlydata", len(bs), bs)
			//}
			thePotentialEarlyData = bs
			return "", true
		}

	}

	var theReader io.Reader
	if optionalFirstBuffer != nil {
		theReader = io.MultiReader(optionalFirstBuffer, underlay)
	} else {
		theReader = underlay
	}

	rw := utils.RW{Reader: theReader, Writer: underlay}

	_, err := theUpgrader.Upgrade(rw)
	if err != nil {

		return httpLayer.FallbackMeta{
			Conn:         underlay,
			H1RequestBuf: optionalFirstBuffer,
			Path:         rp.Path,
			Method:       rp.Method,
		}, httpLayer.ErrShouldFallback
	}

	theConn := &Conn{
		Conn:            underlay,
		underlayIsBasic: netLayer.IsBasicConn(underlay),
		state:           ws.StateServerSide,
		r:               wsutil.NewServerSideReader(underlay),
	}
	//不想客户端；服务端是不怕客户端在握手阶段传来任何多余数据的
	// 因为我们还没实现 0-rtt
	theConn.r.OnIntermediate = wsutil.ControlFrameHandler(underlay, ws.StateServerSide)

	if len(thePotentialEarlyData) > 0 {
		theConn.serverEndGotEarlyData = thePotentialEarlyData
	}

	return theConn, nil
}
