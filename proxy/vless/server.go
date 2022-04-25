package vless

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/url"
	"sync"
	"time"
	"unsafe"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	uuidStr := lc.Uuid
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		userHashes: make(map[[16]byte]bool),
	}

	s.addV2User(&id)

	return s, nil
}

func (ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return NewServer(u)
}
func NewServer(url *url.URL) (proxy.Server, error) {

	uuidStr := url.User.Username()
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		userHashes: make(map[[16]byte]bool),
	}

	s.addV2User(&id)

	return s, nil
}

//Server 同时支持vless v0 和 v1
//实现 proxy.UserServer 以及 tlsLayer.UserHaser
type Server struct {
	proxy.ProxyCommonStruct
	userHashes map[[16]byte]bool
	mux4Hashes sync.RWMutex
}

func (*Server) HasInnerMux() (int, string) {
	return 1, "simplesocks"
}
func (*Server) CanFallback() bool {
	return true
}
func (s *Server) addV2User(u *proxy.V2rayUser) {
	s.userHashes[*u] = true
}

func (s *Server) AddV2User(u *proxy.V2rayUser) {

	s.mux4Hashes.Lock()
	s.userHashes[*u] = true
	s.mux4Hashes.Unlock()
}

func (s *Server) DelV2User(u *proxy.V2rayUser) {

	s.mux4Hashes.RLock()

	hasu := s.userHashes[*u]
	if hasu {
		s.mux4Hashes.RUnlock()
		return
	}

	s.mux4Hashes.Lock()
	delete(s.userHashes, *u)
	s.mux4Hashes.Unlock()

}

func (s *Server) GetUserByBytes(bs []byte) proxy.User {
	if len(bs) < 16 {
		return nil
	}
	thisUUIDBytes := *(*[16]byte)(unsafe.Pointer(&bs[0]))
	if s.userHashes[thisUUIDBytes] {
		return proxy.V2rayUser(thisUUIDBytes)
	}
	return nil
}

func (s *Server) HasUserByBytes(bs []byte) bool {
	if len(bs) < 16 {
		return false
	}
	if s.userHashes[*(*[16]byte)(unsafe.Pointer(&bs[0]))] {
		return true
	}
	return false
}

func (s *Server) UserBytesLen() int {
	return 16
}

func (s *Server) GetUserByStr(str string) proxy.User {
	u, e := utils.StrToUUID(str)
	if e != nil {
		return nil
	}
	return s.GetUserByBytes(u[:])
}

func (s *Server) Name() string { return Name }

// 返回的bytes.Buffer 是用于 回落使用的，内含了整个读取的数据;不回落时不要使用该Buffer
func (s *Server) Handshake(underlay net.Conn) (result net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {

	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		returnErr = err
		return
	}
	defer underlay.SetReadDeadline(time.Time{})

	//这里我们本 不用再创建一个buffer来缓存数据，因为tls包本身就是有缓存的，所以一点一点读就行，tcp本身系统也是有缓存的
	// 因此v1.0.3以及更老版本都是直接一段一段read的。
	//但是，因为需要支持fallback技术，所以还是要 进行缓存

	readbs := utils.GetBytes(utils.MTU)

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
	var use_udp_multi bool

	goto realPart

errorPart:

	//所返回的buffer必须包含所有数据，而Buffer不支持回退，所以只能重新New
	returnErr = &utils.ErrFirstBuffer{
		Err:   returnErr,
		First: bytes.NewBuffer(readbs[:wholeReadLen]),
	}
	return

realPart:
	//这部分过程可以参照 v2ray的 proxy/vless/encoding/encoding.go DecodeRequestHeader 方法
	//see https://github.com/v2fly/v2ray-core/blob/master/proxy/vless/inbound/inbound.go

	auth := readbuf.Next(17)

	version := auth[0]
	if version > 1 {

		returnErr = utils.ErrInErr{ErrDesc: "invalid version ", ErrDetail: utils.ErrInvalidData, Data: version}
		goto errorPart

	}

	idBytes := auth[1:17]

	s.mux4Hashes.RLock()

	thisUUIDBytes := *(*[16]byte)(unsafe.Pointer(&idBytes[0]))

	if s.userHashes[thisUUIDBytes] {
		s.mux4Hashes.RUnlock()
	} else {
		s.mux4Hashes.RUnlock()
		returnErr = utils.ErrInErr{ErrDesc: "invalid user ", ErrDetail: utils.ErrInvalidData, Data: utils.UUIDToStr(thisUUIDBytes)}
		goto errorPart
	}

	if version == 0 {

		addonLenByte, err := readbuf.ReadByte()
		if err != nil {
			returnErr = err
			return
		}
		if addonLenByte != 0 {
			//v2ray的vless中没有对应的任何处理。
			//v2ray 的 vless 虽然有一个没用的Flow，但是 EncodeBodyAddons里根本没向里写任何数据。所以理论上正常这部分始终应该为0
			if ce := utils.CanLogWarn("potential illegal client"); ce != nil {
				ce.Write(zap.Uint8("addonLenByte", addonLenByte))
			}

			if tmpbs := readbuf.Next(int(addonLenByte)); len(tmpbs) != int(addonLenByte) {
				returnErr = errors.New("vless short read in addon")
				return
			}
		}
	} else {
		addonFlagByte, err := readbuf.ReadByte()
		if err != nil {
			returnErr = err
			return
		}

		switch addonFlagByte {
		case addon_udp_multi_flag:
			use_udp_multi = true
		}

	}

	commandByte, err := readbuf.ReadByte()

	if err != nil {

		returnErr = utils.ErrInErr{ErrDesc: "read commandByte failed ", ErrDetail: err}
		goto errorPart
	}

	var isudp, ismux bool

	switch commandByte {
	case CmdMux:

		//verysimple 没有实现 mux.cool,  因为 v2ray的 mux.cool 有很多问题, 本作不会支持v0 的mux

		if version == 0 {
			returnErr = errors.New("mux for vless v0 is not supported by verysimple")
			return // 这个就不回落了.
		} else {
			//v1我们将采用 smux+simplesocks 的方式

			ismux = true

		}
		fallthrough

	case CmdTCP, CmdUDP:

		targetAddr, err = GetAddrFrom(readbuf)
		if err != nil {

			returnErr = utils.ErrInErr{ErrDesc: "fallback, reason 4", ErrDetail: err}
			goto errorPart
		}

		if commandByte == CmdUDP {
			targetAddr.Network = "udp"
			isudp = true
		}

	default:

		returnErr = utils.ErrInErr{ErrDesc: "invalid command ", ErrDetail: utils.ErrInvalidData, Data: commandByte}
		goto errorPart
	}

	if ismux {
		mh := &proxy.MuxMarkerConn{
			ReadWrapper: netLayer.ReadWrapper{
				Conn: underlay,
			},
		}

		if l := readbuf.Len(); l > 0 {
			mh.RemainFirstBufLen = l
			mh.OptionalReader = io.MultiReader(readbuf, underlay)
		}

		return mh, nil, targetAddr, nil
	}

	if isudp {
		return nil, &UDPConn{
			Conn:              underlay,
			version:           int(version),
			raddr:             targetAddr,
			optionalReader:    io.MultiReader(readbuf, underlay),
			remainFirstBufLen: readbuf.Len(),
			udp_multi:         use_udp_multi,
		}, targetAddr, nil

	} else {
		uc := &UserTCPConn{
			Conn:              underlay,
			version:           int(version),
			optionalReader:    io.MultiReader(readbuf, underlay),
			remainFirstBufLen: readbuf.Len(),
			uuid:              thisUUIDBytes,
			underlayIsBasic:   netLayer.IsBasicConn(underlay),
			isServerEnd:       true,
		}

		if r, rr, mr := netLayer.IsConnGoodForReadv(underlay); r > 0 {
			uc.rr = rr
			uc.mr = mr
		}
		return uc, nil, targetAddr, nil

	}

}
