package shadowsocks

import (
	"bytes"
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/shadowsocks/go-shadowsocks2/core"
)

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	uuidStr := lc.Uuid

	var mp MethodPass
	if mp.InitWithStr(uuidStr) {
		return newServer(mp), nil

	}

	return nil, utils.ErrNilOrWrongParameter
}

func (ServerCreator) URLToListenConf(u *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {
	if format != proxy.UrlStandardFormat {
		return lc, utils.ErrUnImplemented
	}
	if lc == nil {
		lc = &proxy.ListenConf{}
	}

	m := u.Query().Get("method")
	p := u.Query().Get("pass")

	lc.Uuid = "method:" + m + "\npass:" + p
	return lc, nil

}

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	cipher core.Cipher
}

func newServer(info MethodPass) *Server {
	return &Server{
		cipher: initShadowCipher(info),
	}
}
func (*Server) Name() string {
	return Name
}

func (*Server) MultiTransportLayer() bool {
	return true
}

func (s *Server) Handshake(underlay net.Conn) (result net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	result = s.cipher.StreamConn(underlay)
	readbs := utils.GetBytes(utils.MTU)

	wholeReadLen, err := result.Read(readbs)
	if err != nil {
		returnErr = utils.ErrInErr{ErrDesc: "read underlay failed", ErrDetail: err, Data: wholeReadLen}
		return
	}

	readbuf := bytes.NewBuffer(readbs[:wholeReadLen])
	goto realPart

errorPart:

	//所返回的 buffer 必须包含所有数据，而 bytes.Buffer 是不支持回退的，所以只能重新 New
	returnErr = &utils.ErrBuffer{
		Err: returnErr,
		Buf: bytes.NewBuffer(readbs[:wholeReadLen]),
	}
	return

realPart:

	targetAddr, err = GetAddrFrom(readbuf)
	if err != nil {
		returnErr = err
		goto errorPart
	}

	result = &netLayer.IOWrapper{
		Reader: &utils.ReadWrapper{
			Reader:            result,
			OptionalReader:    io.MultiReader(readbuf, result),
			RemainFirstBufLen: readbuf.Len(),
		},
		Writer: result,
	}

	return
}
