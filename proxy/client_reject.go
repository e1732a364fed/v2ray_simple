package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const RejectName = "reject"

//implements ClientCreator for reject
type RejectCreator struct{}

func (RejectCreator) NewClientFromURL(url *url.URL) (Client, error) {
	r := &RejectClient{}
	nStr := url.Query().Get("type")
	if nStr != "" {
		r.theType = nStr
	}

	return r, nil
}

func (RejectCreator) NewClient(dc *DialConf) (Client, error) {
	r := &RejectClient{}

	if dc.Extra != nil {
		if thing := dc.Extra["type"]; thing != nil {
			if t, ok := thing.(string); ok && t != "" {
				r.theType = t
			}
		}
	}

	return r, nil
}

/*RejectClient implements Client, optionally response a 403 and close the underlay immediately.

 "blackhole" 名字不准确, 本作 使用 "reject".

正常的 blackhole，并不会立即关闭连接，而是悄无声息地 读 数据，并舍弃。
而 v2ray的 blackhole是 选择性返回 403错误 后立即关闭连接. 完全是 Reject的特性。

而且 理想情况下 应该分析一下请求，如果请求是合法的http请求，则返回403，否则 应该返回 400错误.

所以我们在v2ray的基础上，再推出一个 "nginx"类型，来达到上面的分类返回不同错误的效果。
*/
type RejectClient struct {
	Base

	theType string
}

func (*RejectClient) Name() string { return RejectName }

func (c *RejectClient) tryResponseAndClose(underlay net.Conn) {
	switch c.theType {
	case "http":
		underlay.Write([]byte(httpLayer.Err403response))
	case "nginx":
		SetCommonReadTimeout(underlay)
		bs := utils.GetPacket()
		n, err := underlay.Read(bs)

		if err == nil && n > 0 {
			bs = bs[:n]
			_, _, _, _, failreason := httpLayer.ParseH1Request(bs, false)
			if failreason == 0 {
				underlay.Write([]byte(httpLayer.GetReal403Response()))
			} else {
				underlay.Write([]byte(httpLayer.GetReal400Response()))
			}
		}

	}

	underlay.Close()
}

//optionally response a 403 and close the underlay.
func (c *RejectClient) Handshake(underlay net.Conn, _ []byte, _ netLayer.Addr) (result io.ReadWriteCloser, err error) {
	c.tryResponseAndClose(underlay)
	return nil, io.EOF
}

//function the same as Handshake
func (c *RejectClient) EstablishUDPChannel(underlay net.Conn, _ []byte, _ netLayer.Addr) (netLayer.MsgConn, error) {
	c.tryResponseAndClose(underlay)
	return nil, io.EOF
}
