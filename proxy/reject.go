package proxy

import (
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

const RejectName = "reject"

func tryRejectWithHttpRespAndClose(rejectType string, underlay net.Conn) {
	switch rejectType {
	default:
		if ce := utils.CanLogDebug("reject server got unimplemented rejectType. Use default response instead."); ce != nil {

			ce.Write(
				zap.String("type", rejectType),
			)
		}

		fallthrough
	case "http":
		underlay.Write([]byte(httpLayer.Err403response))
	case "nginx":
		SetCommonReadTimeout(underlay)
		bs := utils.GetPacket()
		defer utils.PutPacket(bs)
		n, err := underlay.Read(bs)

		if err == nil && n > 0 {
			bs = bs[:n]
			_, _, _, _, failreason := httpLayer.ParseH1Request(bs, false)
			if failreason == 0 {
				underlay.Write([]byte(httpLayer.GetNginx403Response())) //forbiden

			} else {

				underlay.Write([]byte(httpLayer.GetNginx400Response())) //bad request, for non-http (illegal) request
			}
		} else {
			if ce := utils.CanLogDebug("reject server got Read error"); ce != nil {

				ce.Write(
					zap.Error(err),
				)
			}
		}
	}

	underlay.Close() //实测，如果不Write 响应，就算Close掉，客户端的连接也不会真正被关闭
}

// implements ClientCreator and ServerCreator for reject
type RejectCreator struct{}

func (RejectCreator) MultiTransportLayer() bool {
	return false
}

func (RejectCreator) NewClient(dc *DialConf) (Client, error) {
	r := &RejectClient{}

	r.initByCommonConf(&dc.CommonConf)

	return r, nil
}

func (rc RejectCreator) URLToDialConf(url *url.URL, iv *DialConf, format int) (*DialConf, error) {
	if iv == nil {
		iv = &DialConf{}

	}

	return iv, nil
}

func (rc RejectCreator) URLToListenConf(url *url.URL, iv *ListenConf, format int) (*ListenConf, error) {
	if iv == nil {
		iv = &ListenConf{}
	}

	return iv, nil
}

func (RejectCreator) NewServer(lc *ListenConf) (Server, error) {
	r := &RejectServer{}

	r.initByCommonConf(&lc.CommonConf)

	return r, nil
}

type rejectCommon struct {
	Base

	theType string //拒绝响应的类型, 可为空、http或nginx
}

func (*rejectCommon) Name() string { return RejectName }

func (rc *rejectCommon) initByCommonConf(cc *CommonConf) {
	if cc.Extra != nil {
		if thing := cc.Extra["type"]; thing != nil {
			if t, ok := thing.(string); ok && t != "" {
				rc.theType = t
			}
		}
	}

}

/*
RejectClient implements Client, optionally response a 403 and close the underlay immediately.

	v2ray的 "blackhole" 名字不准确, 本作 使用 "reject".

正常的 blackhole，并不会立即关闭连接，而是悄无声息地 读 数据，并舍弃。
而 v2ray的 blackhole是 选择性返回 403错误 后立即关闭连接. 完全是 Reject的特性。

而且 理想情况下 应该分析一下请求，如果请求是合法的http请求，则返回403，否则 应该返回 400错误.

所以我们在v2ray的基础上，再推出一个 "nginx"类型，来达到上面的分类返回不同错误的效果。

默认为 "" 空类型，直接 close，不反回任何信息。 若设为 http，则返回一个403错误；若设为nginx，则分类返回400/403错误。
*/
type RejectClient struct {
	rejectCommon
}

// optionally response 403 and close the underlay, return io.EOF.
func (c *RejectClient) Handshake(underlay net.Conn, _ []byte, _ netLayer.Addr) (result io.ReadWriteCloser, err error) {
	tryRejectWithHttpRespAndClose(c.theType, underlay)
	return nil, io.EOF
}

// function the same as Handshake
func (c *RejectClient) EstablishUDPChannel(underlay net.Conn, _ []byte, _ netLayer.Addr) (netLayer.MsgConn, error) {
	tryRejectWithHttpRespAndClose(c.theType, underlay)
	return nil, io.EOF
}

// mimic the behavior of RejectClient
type RejectServer struct {
	rejectCommon
}

// return utils.ErrHandled
func (s *RejectServer) Handshake(underlay net.Conn) (_ net.Conn, _ netLayer.MsgConn, _ netLayer.Addr, e error) {
	tryRejectWithHttpRespAndClose(s.theType, underlay)

	e = utils.ErrHandled
	return
}
