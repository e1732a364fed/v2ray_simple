package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
)

const rejectName = "reject"

func init() {
	RegisterClient(rejectName, &RejectCreator{})
}

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

//实现了 Client, 选择性返回http403错误, 然后立即关闭连接。
//
// "blackhole" 名字不准确，verysimple将使用 "reject"
//
//正常的 blackhole，并不会立即关闭连接，而是悄无声息地 读 数据，并舍弃。
//而 v2ray的 blackhole是 选择性返回 403错误 后立即关闭连接. 完全是 Reject的特性。
type RejectClient struct {
	ProxyCommonStruct

	theType string
}

func (*RejectClient) Name() string { return rejectName }

func (b *RejectClient) tryResponseAndClose(underlay net.Conn) {
	switch b.theType {
	case "http":
		underlay.Write([]byte(httpLayer.Err403response))
	}

	underlay.Close()
}

func (b *RejectClient) Handshake(underlay net.Conn, _ []byte, _ netLayer.Addr) (result io.ReadWriteCloser, err error) {
	b.tryResponseAndClose(underlay)
	return nil, io.EOF
}

func (b *RejectClient) EstablishUDPChannel(underlay net.Conn, _ netLayer.Addr) (netLayer.MsgConn, error) {
	b.tryResponseAndClose(underlay)
	return nil, io.EOF
}
