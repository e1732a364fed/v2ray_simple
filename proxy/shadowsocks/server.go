package shadowsocks

import (
	"bytes"
	"io"
	"net"
	"net/url"
	"sync"

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

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	cipher core.Cipher

	m             sync.RWMutex
	udpMsgConnMap map[netLayer.HashableAddr]*shadowUDPPacketConn
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

func (s *Server) SelfListen() (is, tcp, udp bool) {
	udp = true
	is = true
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

func (ss *Server) StartListen(_ chan<- proxy.IncomeTCPInfo, udpInfoChan chan<- proxy.IncomeUDPInfo) io.Closer {
	// uc, err := net.ListenUDP("udp", ss.LUA)
	// if err != nil {
	// 	log.Panicln("shadowsocks listen udp failed", err)
	// }
	// pc := ss.cipher.PacketConn(uc)

	// sp := &shadowUDPPacketConn{
	// 	PacketConn: pc,
	// }

	go func() {

		// for {
		// 	bs := utils.GetPacket()

		// 	n, addr, err := pc.ReadFrom(bs)
		// 	if err != nil {
		// 		return
		// 	}
		// 	ad, err := netLayer.NewAddrFromAny(addr)
		// 	if err != nil {
		// 		return
		// 	}
		// 	hash := ad.GetHashable()

		// 	ss.m.RLock()
		// 	conn, found := ss.udpMsgConnMap[hash]
		// 	ss.m.RUnlock()

		// 	if !found {
		// 		conn = &shadowUDPPacketConn{
		// 			ourSrcAddr:    src,
		// 			readChan:      make(chan netLayer.AddrData, 5),
		// 			closeChan:     make(chan struct{}),
		// 			parentMachine: m,
		// 			hash:          hash,
		// 		}
		// 		conn.InitEasyDeadline()

		// 		m.Lock()
		// 		m.udpMsgConnMap[hash] = conn
		// 		m.Unlock()

		// 	}

		// 	destAddr := netLayer.NewAddrFromUDPAddr(dst)

		// 	conn.readChan <- netLayer.AddrData{Data: bs[:n], Addr: destAddr}

		// 	if !found {
		// 		return conn, destAddr, nil

		// 	}

		// }

	}()
	return nil

}
