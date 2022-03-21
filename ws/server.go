package ws

import (
	"net"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/utils"
)

// Handshake 用于 websocket的 Server 监听端，建立握手. 用到了 gobwas/ws.Upgrader.
//
// 这里默认: 传入的path必须 以 "/" 为前缀. 本函数 不对此进行任何检查.
//
// 返回可直接用于读写 websocket 二进制数据的 net.Conn
func Handshake(path string, underlay net.Conn) (net.Conn, error) {

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

	_, err := upgrader.Upgrade(underlay)
	if err != nil {
		return nil, err
	}

	theConn := &Conn{
		Conn:  underlay,
		state: ws.StateServerSide,
		//w:     wsutil.NewWriter(underlay, ws.StateServerSide, ws.OpBinary),
		r: wsutil.NewServerSideReader(underlay),
	}
	//不想客户端；服务端是不怕客户端在握手阶段传来任何多余数据的
	// 因为我们还没实现 0-rtt
	theConn.r.OnIntermediate = wsutil.ControlFrameHandler(underlay, ws.StateServerSide)

	return theConn, nil
}
