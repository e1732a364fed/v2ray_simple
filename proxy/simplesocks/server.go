package simplesocks

import (
	"bytes"
	"io"
	"net"
	"net/url"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{ proxy.CreatorCommonStruct }

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	s := &Server{}
	return s, nil
}

func (ServerCreator) URLToListenConf(u *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {
	if lc == nil {
		lc = &proxy.ListenConf{}
	}
	return lc, nil
}

// implements proxy.Server
type Server struct {
	proxy.Base
}

func (*Server) Name() string {
	return Name
}
func (*Server) CanFallback() bool {
	return true //simplesocks理论上当然是支持回落的，但是一般它被用于 innerMux的内层协议，所以用做innerMux内层协议时，要注意不要再回落了。
}

// 若握手步骤数据不对, 会返回 ErrDetail 为 utils.ErrInvalidData 的 utils.ErrInErr
func (s *Server) Handshake(underlay net.Conn) (result net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	if err := proxy.SetCommonReadTimeout(underlay); err != nil {
		returnErr = err
		return
	}
	defer netLayer.PersistConn(underlay)

	readbs := utils.GetBytes(utils.MTU)

	wholeReadLen, err := underlay.Read(readbs)
	if err != nil {
		returnErr = utils.ErrInErr{ErrDesc: "Err, simplesocks handshake, read underlay", ErrDetail: err, Data: wholeReadLen}
		return
	}

	if wholeReadLen < 4 {
		returnErr = utils.ErrInErr{ErrDesc: "Err, simplesocks, msg too short", Data: wholeReadLen}
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

	cmdb, _ := readbuf.ReadByte()

	var isudp bool
	switch cmdb {
	default:
		returnErr = utils.ErrInErr{ErrDesc: "Err, simplesocks, cmd byte wrong", ErrDetail: utils.ErrInvalidData, Data: cmdb}
		goto errorPart
	case CmdTCP:

	case CmdUDP:
		isudp = true

	}

	targetAddr, err = GetAddrFrom(readbuf)
	if err != nil {
		returnErr = err
		goto errorPart
	}
	if isudp {
		targetAddr.Network = "udp"
	}

	if isudp {
		x := NewUDPConn(underlay, io.MultiReader(readbuf, underlay))
		x.fullcone = s.IsFullcone
		return nil, x, targetAddr, nil

	} else {
		return &TCPConn{
			Conn:              underlay,
			optionalReader:    io.MultiReader(readbuf, underlay),
			remainFirstBufLen: readbuf.Len(),
			underlayIsBasic:   netLayer.IsBasicConn(underlay),
			isServerEnd:       true,
		}, nil, targetAddr, nil

	}
}
