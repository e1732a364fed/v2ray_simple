package trojan

import (
	"bytes"
	"errors"
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

type Server struct {
	proxy.ProxyCommonStruct

	userHashes map[string]bool

	//mux4Hashes sync.RWMutex
}
type ServerCreator struct{}

func (_ ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	uuidStr := lc.Uuid

	s := &Server{
		userHashes: make(map[string]bool),
	}

	s.userHashes[string(SHA224_hexStringBytes(uuidStr))] = true

	return s, nil
}

func (_ ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return nil, utils.ErrNotImplemented
}
func (s *Server) Name() string {
	return Name
}

func (s *Server) Handshake(underlay net.Conn) (result io.ReadWriteCloser, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		returnErr = err
		return
	}
	defer underlay.SetReadDeadline(time.Time{})

	readbs := utils.GetBytes(utils.StandardBytesLength)

	wholeReadLen, err := underlay.Read(readbs)
	if err != nil {
		returnErr = utils.ErrInErr{ErrDesc: "read err", ErrDetail: err, Data: wholeReadLen}
		return
	}

	if wholeReadLen < 17 {
		//根据下面回答，HTTP的最小长度恰好是16字节，但是是0.9版本。1.0是18字节，1.1还要更长。总之我们可以直接不返回fallback地址
		//https://stackoverflow.com/questions/25047905/http-request-minimum-size-in-bytes/25065089

		returnErr = utils.ErrInErr{ErrDesc: "fallback, msg too short", Data: wholeReadLen}
		return
	}

	readbuf := bytes.NewBuffer(readbs[:wholeReadLen])

	goto realPart

errorPart:

	//所返回的buffer必须包含所有数据，而Buffer不支持回退，所以只能重新New
	returnErr = &utils.ErrFirstBuffer{
		Err:   returnErr,
		First: bytes.NewBuffer(readbs[:wholeReadLen]),
	}
	return

realPart:

	if wholeReadLen < 56+8+1 {
		returnErr = utils.ErrInvalidData
		goto errorPart
	}

	//可参考 https://github.com/p4gefau1t/trojan-go/blob/master/tunnel/trojan/server.go

	hash := readbuf.Next(56)
	hashStr := string(hash)
	if !s.userHashes[hashStr] {
		returnErr = errors.New("hash not match")
		goto errorPart
	}

	crb, _ := readbuf.ReadByte()
	lfb, _ := readbuf.ReadByte()
	if crb != crlf[0] || lfb != crlf[1] {
		returnErr = utils.ErrInvalidData
		goto errorPart
	}

	cmdb, _ := readbuf.ReadByte()

	var isudp bool
	switch cmdb {
	default:
		returnErr = utils.ErrInvalidData
		goto errorPart
	case CmdConnect:

	case CmdUDPAssociate:
		isudp = true
	}

	targetAddr, err = GetAddrFromReader(readbuf)
	if err != nil {
		returnErr = err
		goto errorPart
	}
	if isudp {
		targetAddr.Network = "udp"
	}
	crb, err = readbuf.ReadByte()
	if err != nil {
		returnErr = err
		goto errorPart
	}
	lfb, err = readbuf.ReadByte()
	if err != nil {
		returnErr = err
		goto errorPart
	}
	if crb != crlf[0] || lfb != crlf[1] {
		returnErr = utils.ErrInvalidData
		goto errorPart
	}

	if isudp {
		return nil, NewUDPConn(underlay, io.MultiReader(readbuf, underlay)), targetAddr, nil

	} else {
		if readbuf.Len() == 0 {
			return underlay, nil, targetAddr, nil
		} else {
			return &UserTCPConn{
				Conn:              underlay,
				optionalReader:    io.MultiReader(readbuf, underlay),
				remainFirstBufLen: readbuf.Len(),
				hash:              hashStr,
				underlayIsBasic:   netLayer.IsBasicConn(underlay),
				isServerEnd:       true,
			}, nil, targetAddr, nil
		}

	}
}
