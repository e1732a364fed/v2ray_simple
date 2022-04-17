package simplesocks

import (
	"bytes"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	s := &Server{}
	return s, nil
}

func (ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	s := &Server{}
	return s, nil
}

//implements proxy.Server
type Server struct {
	proxy.ProxyCommonStruct
}

func (*Server) Name() string {
	return Name
}

//若握手步骤数据不对, 会返回 ErrDetail 为 utils.ErrInvalidData 的 utils.ErrInErr
func (s *Server) Handshake(underlay net.Conn) (result net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		returnErr = err
		return
	}
	defer underlay.SetReadDeadline(time.Time{})

	readbs := utils.GetBytes(utils.StandardBytesLength)

	wholeReadLen, err := underlay.Read(readbs)
	if err != nil {
		returnErr = utils.ErrInErr{ErrDesc: "read underlay failed", ErrDetail: err, Data: wholeReadLen}
		return
	}

	if wholeReadLen < 4 {
		returnErr = utils.ErrInErr{ErrDesc: "fallback, msg too short", Data: wholeReadLen}
		return
	}

	readbuf := bytes.NewBuffer(readbs[:wholeReadLen])

	goto realPart

errorPart:

	//所返回的 buffer 必须包含所有数据，而 bytes.Buffer 是不支持回退的，所以只能重新 New
	returnErr = &utils.ErrFirstBuffer{
		Err:   returnErr,
		First: bytes.NewBuffer(readbs[:wholeReadLen]),
	}
	return

realPart:

	cmdb, _ := readbuf.ReadByte()

	var isudp bool
	switch cmdb {
	default:
		returnErr = utils.ErrInErr{ErrDesc: "cmd byte wrong", ErrDetail: utils.ErrInvalidData, Data: cmdb}
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
		return nil, NewUDPConn(underlay, io.MultiReader(readbuf, underlay)), targetAddr, nil

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
