package ws

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/url"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// 注意，Client并不实现 proxy.Client.
// Client只是在tcp/tls 的基础上包了一层websocket而已，不管其他内容.
// 而 proxy.Client 是要 把 需要代理的 真实目标 地址 以某种方式 写入 数据内容的.
// 这也是 我们ws包 并没有被 放在proxy文件夹中 的原因
type Client struct {
	requestURL *url.URL //因为调用gobwas/ws.Dialer.Upgrade 时要传入url，所以我们直接提供包装好的即可
}

// 这里默认，传入的path必须 以 "/" 为前缀. 本函数 不对此进行任何检查
func NewClient(hostAddr, path string) (*Client, error) {
	u, err := url.Parse("http://" + hostAddr + path)
	if err != nil {
		return nil, err
	}
	return &Client{
		requestURL: u,
	}, nil
}

//与服务端进行 websocket握手，并返回可直接用于读写 websocket 二进制数据的 net.Conn
func (c *Client) Handshake(underlay net.Conn) (net.Conn, error) {

	//实测默认的4096过小,因为 实测 tls握手的serverHello 就有可能超过了4096,
	// 但是仔细思考，发现tls握手是在websocket的外部发生的，而我们传输的是数据的内层tls握手，那么就和Dialer没关系了，dialer只是负责读最初的握手部分；
	// 所以我们就算要配置buffer尺寸，也不是在这里配置，而是要配置 theConn.w 的buffer

	//const bufsize = 1024 * 10
	d := ws.Dialer{
		//ReadBufferSize:  bufsize,
		//WriteBufferSize: bufsize,
		NetDial: func(ctx context.Context, net, addr string) (net.Conn, error) {
			return underlay, nil
		},
	}

	br, _, err := d.Upgrade(underlay, c.requestURL)
	if err != nil {
		return nil, err
	}
	//之所以返回值中有br，是因为服务器可能紧接着向我们迅猛地发送数据;
	//
	// 但是，我们代理的方法是先等待握手成功再传递数据，而且每次都是客户端先传输数据,
	// 所以我们的用例中，br一定是nil

	theConn := &Conn{
		Conn:  underlay,
		state: ws.StateClientSide,
		//w:     wsutil.NewWriter(underlay, ws.StateClientSide, ws.OpBinary),
	}
	//theConn.w.DisableFlush() //发现使用ws分片功能的话会出问题，所以就先关了. 搞清楚分片的问题再说。

	// 根据 gobwas/ws的代码，在服务器没有返回任何数据时，br为nil
	if br == nil {
		theConn.r = wsutil.NewClientSideReader(underlay)

		theConn.r.OnIntermediate = wsutil.ControlFrameHandler(underlay, ws.StateClientSide)
		// OnIntermediate 会在 r.NextFrame 里被调用. 如果我们不在这里提供，就要每次都在Read里操作，多此一举

		return theConn, nil
	}

	//从bufio.Reader中提取出剩余读到的部分, 与underlay生成一个MultiReader

	additionalDataNum := br.Buffered()
	bs, _ := br.Peek(additionalDataNum)

	wholeR := io.MultiReader(bytes.NewBuffer(bs), underlay)

	theConn.r = wsutil.NewClientSideReader(wholeR)
	theConn.r.OnIntermediate = wsutil.ControlFrameHandler(underlay, ws.StateClientSide)

	return theConn, nil
}
