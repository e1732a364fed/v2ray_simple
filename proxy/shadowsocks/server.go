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

type ServerCreator struct{ proxy.CreatorCommonStruct }

func (ServerCreator) MultiTransportLayer() bool {
	return true
}
func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {

	if lc.Network == "" {
		lc.Network = netLayer.DualNetworkName
	}

	uuidStr := lc.Uuid

	var mp MethodPass
	if mp.InitWithStr(uuidStr) {
		return newServer(mp, lc), nil

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

	if p, set := u.User.Password(); set {
		lc.Uuid = "method:" + u.User.Username() + "\npass:" + p
	}

	return lc, nil

}

// loop read udp
// func (ServerCreator) AfterCommonConfServer(s proxy.Server) {
// 	if s.Network() == "udp" || s.Network() == netLayer.DualNetworkName {
// 		ss := s.(*Server)
// 		uc, err := net.ListenUDP("udp", ss.LUA)
// 		if err != nil {
// 			log.Panicln("shadowsocks listen udp failed", err)
// 		}
// 		pc := ss.cipher.PacketConn(uc)
// 		for {
// 			buf := utils.GetPacket()
// 			defer utils.PutPacket(buf)
// 			n, saddr, err := pc.ReadFrom(buf)
// 			if err != nil {
// 				if ce := utils.CanLogErr("shadowsocks read udp failed"); ce != nil {
// 					ce.Write(zap.Error(err))
// 				}
// 				return
// 			}
// 			r := bytes.NewBuffer(buf[:n])
// 			taddr, err := GetAddrFrom(r)
// 			if err != nil {
// 				if ce := utils.CanLogErr("shadowsocks GetAddrFrom failed"); ce != nil {
// 					ce.Write(zap.Error(err))
// 				}
// 				return
// 			}

// 		}
// 	}
// }

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	cipher core.Cipher
}

func newServer(info MethodPass, lc *proxy.ListenConf) *Server {
	s := &Server{
		cipher: initShadowCipher(info),
	}

	return s
}
func (*Server) Name() string {
	return Name
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
