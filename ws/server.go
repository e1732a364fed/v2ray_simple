package ws

import (
	"encoding/base64"
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/utils"
)

// 2048 /3 = 682.6666 ,
// 683 * 4 = 2732
const MaxEarlyDataLen_Base64 = 2732

type Server struct {
	upgrader     *ws.Upgrader
	UseEarlyData bool
}

// 这里默认: 传入的path必须 以 "/" 为前缀. 本函数 不对此进行任何检查.
func NewServer(path string) *Server {
	//因为我们vs的架构，先统一监听tcp；然后再调用Handshake函数
	// 所以我们不能直接用http.Handle, 这也彰显了 用 gobwas/ws 包的好处
	// 给Upgrader提供的 OnRequest 专门用于过滤 path, 也不需要我们的 httpLayer 去过滤

	// 我们的 httpLayer 的 过滤方法仍然是最安全的，可以杜绝 所有非法数据；
	// 而 ws.Upgrader.Upgrade 使用了 readLine 函数。如果客户提供一个非法的超长的一行的话，它就会陷入泥淖
	// 这个以后 可以先用 httpLayer的过滤方法，过滤掉后，再用 MultiReader组装回来，提供给 upgrader.Upgrade
	// ReadBufferSize默认是 4096，已经够大

	upgrader := &ws.Upgrader{
		OnRequest: func(uri []byte) error {
			struri := string(uri)
			if struri != path {
				min := len(path)
				if len(struri) < min {
					min = len(struri)
				}
				return utils.NewDataErr("ws path not match", nil, struri[:min])
			}
			return nil
		},
	}
	return &Server{
		upgrader: upgrader,
	}
}

// Handshake 用于 websocket的 Server 监听端，建立握手. 用到了 gobwas/ws.Upgrader.
//
// 返回可直接用于读写 websocket 二进制数据的 net.Conn
func (s *Server) Handshake(underlay net.Conn) (net.Conn, error) {

	var theUpgrader *ws.Upgrader = s.upgrader

	var thePotentialEarlyData []byte

	if s.UseEarlyData {
		newupgrader := &ws.Upgrader{
			OnRequest: s.upgrader.OnRequest,

			//xray和v2ray中，使用了
			// Sec-WebSocket-Protocol 字段 来传输 earlydata，来实现 0-rtt
			// websocket标准是没有定义 0-rtt的方法的，但是ws的握手包头部是可以自定义header的
			// gobwas 的upgrader 用 ProtocolCustom 这个函数来检查 protocol的内容

			// 它会遍历客户端给出的所有 protocol，然后选择一个来返回

			//我们若提供了此函数，则必须返回true，否则 gobwas会返回 ErrMalformedRequest 错误

			ProtocolCustom: func(b []byte) (string, bool) {
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
			},
		}

		theUpgrader = newupgrader
	}

	_, err := theUpgrader.Upgrade(underlay)
	if err != nil {
		return nil, err
	}

	//log.Println("thePotentialEarlyData", len(thePotentialEarlyData))

	theConn := &Conn{
		Conn:  underlay,
		state: ws.StateServerSide,
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
