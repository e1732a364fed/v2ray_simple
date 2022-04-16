package ws

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/url"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

//为了避免黑客攻击,我们固定earlydata最大值为2048
const MaxEarlyDataLen = 2048

// 注意，Client并不实现 proxy.Client.
// Client只是在tcp/tls 的基础上包了一层websocket而已，不管其他内容.
// 而 proxy.Client 是要 把 需要代理的 真实目标 地址 以某种方式 写入 数据内容的.
// 这也是 我们ws包 并没有被 放在proxy文件夹中 的原因
type Client struct {
	requestURL   *url.URL //因为调用gobwas/ws.Dialer.Upgrade 时要传入url，所以我们直接提供包装好的即可
	UseEarlyData bool
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

	d := ws.Dialer{
		NetDial: func(ctx context.Context, net, addr string) (net.Conn, error) {
			return underlay, nil
		},
		// 默认不给出Protocols的话, gobwas就不会发送这个header, 另一端也收不到此header
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
		Conn:            underlay,
		state:           ws.StateClientSide,
		underlayIsBasic: netLayer.IsBasicConn(underlay),
	}

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

// 我们要先返回一个 Conn, 然后读取到内层的 vless等协议的握手后，再进行实际的 ws握手
func (c *Client) HandshakeWithEarlyData(underlay net.Conn, ed []byte) (net.Conn, error) {

	return &EarlyDataConn{
		Conn: underlay,

		earlyData:            ed,
		requestURL:           c.requestURL,
		firstHandshakeOkChan: make(chan int, 1),
		dialer: &ws.Dialer{
			NetDial: func(ctx context.Context, net, addr string) (net.Conn, error) {
				return underlay, nil
			},
		},
	}, nil
}

type EarlyDataConn struct {
	net.Conn
	dialer     *ws.Dialer
	realWsConn net.Conn

	notFirst             bool
	earlyData            []byte
	requestURL           *url.URL
	firstHandshakeOkChan chan int

	notFirstRead bool
}

//第一次会获取到 内部的包头, 然后我们在这里才开始执行ws的握手
// 这是verysimple的架构造成的. ws层后面跟着的应该就是代理层 的 Handshake调用,它会写入一次包头
// 我们就是利用这个特征, 把vless包头 和 之前给出的earlydata绑在一起，进行base64编码然后进行ws握手
func (edc *EarlyDataConn) Write(p []byte) (int, error) {

	if !edc.notFirst {
		edc.notFirst = true

		outBuf := utils.GetBuf()
		encoder := base64.NewEncoder(base64.RawURLEncoding, outBuf)

		multiReader := io.MultiReader(bytes.NewReader(p), bytes.NewReader(edc.earlyData))
		_, encerr := io.Copy(encoder, multiReader)
		if encerr != nil {
			close(edc.firstHandshakeOkChan)
			return 0, utils.ErrInErr{ErrDesc: "encode early data err", ErrDetail: encerr}
		}
		encoder.Close()

		edc.dialer.Protocols = []string{outBuf.String()}

		br, _, err := edc.dialer.Upgrade(edc.Conn, edc.requestURL)
		if err != nil {
			close(edc.firstHandshakeOkChan)
			return 0, err
		}

		utils.PutBuf(outBuf)

		theConn := &Conn{
			Conn:            edc.Conn,
			state:           ws.StateClientSide,
			underlayIsBasic: netLayer.IsBasicConn(edc.Conn),
		}

		//实测总是 br==nil，就算发送了earlydata也是如此;不过理论上有可能粘包,只要远程目标服务器的响应够快

		if br == nil {
			theConn.r = wsutil.NewClientSideReader(edc.Conn)

			theConn.r.OnIntermediate = wsutil.ControlFrameHandler(edc.Conn, ws.StateClientSide)

		} else {

			additionalDataNum := br.Buffered()
			bs, _ := br.Peek(additionalDataNum)

			wholeR := io.MultiReader(bytes.NewBuffer(bs), edc.Conn)

			theConn.r = wsutil.NewClientSideReader(wholeR)
			theConn.r.OnIntermediate = wsutil.ControlFrameHandler(edc.Conn, ws.StateClientSide)

		}

		edc.realWsConn = theConn
		edc.firstHandshakeOkChan <- 1
		return len(p), nil

	} //if !edc.notFirst {

	return edc.realWsConn.Write(p)
}

func (edc *EarlyDataConn) Read(p []byte) (int, error) {
	if !edc.notFirstRead {
		_, ok := <-edc.firstHandshakeOkChan
		if !ok {
			return 0, errors.New("EarlyDataConn read failed because handshake failed")
		}
		edc.notFirstRead = true

	}
	return edc.realWsConn.Read(p)
}
