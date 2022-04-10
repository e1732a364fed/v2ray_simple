package ws

import (
	"bytes"
	"encoding/base64"
	"io"
	"net"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

// 2048 /3 = 682.6666 ,
// 683 * 4 = 2732, 若你不信，运行 we_test.go中的 TestBase64Len
const MaxEarlyDataLen_Base64 = 2732

type Server struct {
	//upgrader     *ws.Upgrader
	UseEarlyData bool
	Thepath      string
}

// 这里默认: 传入的path必须 以 "/" 为前缀. 本函数 不对此进行任何检查.
func NewServer(path string) *Server {

	return &Server{
		//upgrader: upgrader,
		Thepath: path,
	}
}

// Handshake 用于 websocket的 Server 监听端，建立握手. 用到了 gobwas/ws.Upgrader.
//
// 返回可直接用于读写 websocket 二进制数据的 net.Conn
func (s *Server) Handshake(optionalFirstBuffer *bytes.Buffer, underlay net.Conn) (net.Conn, error) {

	theWrongPath := ""
	var thePotentialEarlyData []byte

	var theUpgrader *ws.Upgrader = &ws.Upgrader{

		//因为我们vs的架构，先统一监听tcp；然后再调用Handshake函数
		// 所以我们不能直接用http.Handle, 这也彰显了 用 gobwas/ws 包的好处
		// 给Upgrader提供的 OnRequest 专门用于过滤 path, 也不需要我们的 httpLayer 去过滤

		// 我们的 httpLayer 的 过滤方法仍然是最安全的，可以杜绝 所有非法数据；
		// 而 ws.Upgrader.Upgrade 使用了 readLine 函数。如果客户提供一个非法的超长的一行的话，它就会陷入泥淖
		// 这个以后 可以先用 httpLayer的过滤方法，过滤掉后，再用 MultiReader组装回来，提供给 upgrader.Upgrade
		// ReadBufferSize默认是 4096，已经够大

		OnRequest: func(uri []byte) error {
			struri := string(uri)
			if struri != s.Thepath {

				theWrongPath = struri

				//return utils.NewDataErr("ws path not match", nil, struri[:min])
				//发现这个错误除了在程序里返回外，还会直接显示到 浏览器上！这会被探测到的。
				// 所以只能显示标准http错误, 然后通过闭包的方式 把path信息传递到外部.
				if ce := utils.CanLogWarn("ws path not match"); ce != nil {
					min := len(s.Thepath)
					if len(struri) < min {
						min = len(struri)
					}
					//log.Println("ws path not match", struri[:min])
					ce.Write(zap.String("wrong path", struri[:min]))
				}
				return ws.RejectConnectionError(ws.RejectionStatus(http.StatusBadRequest))
			}
			return nil
		},
	}

	if s.UseEarlyData {

		//xray和v2ray中，使用了 header中的
		// Sec-WebSocket-Protocol 字段 来传输 earlydata，来实现 0-rtt;我们为了兼容同样用此字段
		// (websocket标准是没有定义 0-rtt的方法的，但是ws的握手包头部是可以自定义header的)

		// gobwas 的upgrader 用 ProtocolCustom 这个函数来检查 protocol的内容
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
		if len(theWrongPath) > 0 {
			//ws的Method必为Get, 否则 gobwas 会返回 ErrHandshakeBadMethod
			return nil, &httpLayer.RequestErr{Path: theWrongPath}
		}
		return nil, err
	}

	//log.Println("thePotentialEarlyData", len(thePotentialEarlyData))

	theConn := &Conn{
		Conn:            underlay,
		underlayIsBasic: netLayer.IsBasicConn(underlay),
		state:           ws.StateServerSide,
		//w:     wsutil.NewWriter(underlay, ws.StateServerSide, ws.OpBinary),
		r: wsutil.NewServerSideReader(underlay),
	}
	//不想客户端；服务端是不怕客户端在握手阶段传来任何多余数据的
	// 因为我们还没实现 0-rtt
	theConn.r.OnIntermediate = wsutil.ControlFrameHandler(underlay, ws.StateServerSide)

	if len(thePotentialEarlyData) > 0 {
		theConn.serverEndGotEarlyData = thePotentialEarlyData
	}

	return theConn, nil
}
