package shadowsocks

import (
	"bytes"
	"io"
	"log"
	"net"
	"net/url"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/shadowsocks/go-shadowsocks2/core"
	"go.uber.org/zap"
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

func (ServerCreator) AfterCommonConfServer(ps proxy.Server) (err error) {
	s := ps.(*Server)

	if s.TransportLayer != "tcp" {

		s.LUA, err = net.ResolveUDPAddr("udp", s.AddrStr())
	}
	return
}

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	cipher core.Cipher

	m             sync.RWMutex
	udpMsgConnMap map[netLayer.HashableAddr]*serverMsgConn
}

func newServer(info MethodPass, lc *proxy.ListenConf) *Server {
	s := &Server{
		cipher:        initShadowCipher(info),
		udpMsgConnMap: make(map[netLayer.HashableAddr]*serverMsgConn),
	}

	return s
}
func (*Server) Name() string {
	return Name
}

func (s *Server) Network() string {
	if s.TransportLayer == "" {
		return netLayer.DualNetworkName
	} else {
		return s.TransportLayer
	}
}

func (s *Server) SelfListen() (is bool, _, udp int) {
	switch n := s.Network(); n {
	case "", netLayer.DualNetworkName:
		udp = 1

	case "tcp":

	case "udp":
		udp = 1
	}

	is = udp > 0

	return
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

func (m *Server) removeUDPByHash(hash netLayer.HashableAddr) {
	m.Lock()
	delete(m.udpMsgConnMap, hash)
	m.Unlock()
}

// 非阻塞
func (s *Server) StartListen(_ func(netLayer.TCPRequestInfo), udpFunc func(netLayer.UDPRequestInfo)) io.Closer {
	uc, err := net.ListenUDP("udp", s.LUA)
	if err != nil {
		log.Panicln("shadowsocks listen udp failed", err)
	}
	pc := s.cipher.PacketConn(uc)

	if ce := utils.CanLogInfo("shadowsocks listening udp"); ce != nil {
		ce.Write(zap.String("listen addr", s.LUA.String()))
	}
	//逻辑完全类似tproxy，使用一个map存储不同终端的链接
	go func() {

		for {
			bs := utils.GetPacket()

			n, addr, err := pc.ReadFrom(bs)
			if err != nil {
				return
			}
			ad, err := netLayer.NewAddrFromAny(addr)
			if err != nil {
				if ce := utils.CanLogWarn("shadowsocks GetAddrFrom err"); ce != nil {
					ce.Write(zap.Error(err))
				}
				return
			}
			hash := ad.GetHashable()

			s.m.RLock()
			conn, found := s.udpMsgConnMap[hash]
			s.m.RUnlock()

			if !found {
				conn = &serverMsgConn{
					raddr:         addr,
					ourPacketConn: pc,
					readChan:      make(chan netLayer.AddrData, 5),
					closeChan:     make(chan struct{}),
					server:        s,
					hash:          hash,
				}
				conn.InitEasyDeadline()

				s.m.Lock()
				s.udpMsgConnMap[hash] = conn
				s.m.Unlock()

			}

			readbuf := bytes.NewBuffer(bs[:n])

			destAddr, err := GetAddrFrom(readbuf)
			if err != nil {
				if ce := utils.CanLogWarn("shadowsocks GetAddrFrom err"); ce != nil {
					ce.Write(zap.Error(err))
				}
				continue
			}
			destAddr.Network = "udp"

			conn.readChan <- netLayer.AddrData{Data: readbuf.Bytes(), Addr: destAddr}

			if !found {

				go udpFunc(netLayer.UDPRequestInfo{
					MsgConn: conn, Target: destAddr,
				})
			}

		}

	}()
	return uc

}
