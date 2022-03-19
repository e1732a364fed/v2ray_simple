package vless

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"sync"
	"time"
	"unsafe"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

//实现 proxy.UserServer 以及 tlsLayer.UserHaser
type Server struct {
	proxy.ProxyCommonStruct
	userHashes   map[[16]byte]*proxy.V2rayUser
	userCRUMFURS map[[16]byte]*CRUMFURS
	mux4Hashes   sync.RWMutex

	defaultfallbackAddr *netLayer.Addr
}

type ServerCreator struct{}

func (_ ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	uuidStr := lc.Uuid
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: lc.GetAddr()},
		userHashes:        make(map[[16]byte]*proxy.V2rayUser),
		userCRUMFURS:      make(map[[16]byte]*CRUMFURS),
	}

	fallbackStr := lc.Fallback

	if fallbackStr != "" {
		fa, err := netLayer.NewAddr(fallbackStr)

		if err != nil {
			return nil, fmt.Errorf("invalid fallback %v", fallbackStr)
		}

		s.defaultfallbackAddr = fa
	}

	s.addV2User(id)

	return s, nil
}

func (_ ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {
	return NewServer(u)
}
func NewServer(url *url.URL) (proxy.Server, error) {

	addr := url.Host
	uuidStr := url.User.Username()
	id, err := proxy.NewV2rayUser(uuidStr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		ProxyCommonStruct: proxy.ProxyCommonStruct{Addr: addr},
		userHashes:        make(map[[16]byte]*proxy.V2rayUser),
		userCRUMFURS:      make(map[[16]byte]*CRUMFURS),
	}

	s.ProxyCommonStruct.InitFromUrl(url)

	query := url.Query()
	fallbackStr := query.Get("fallback")

	if fallbackStr != "" {
		fa, err := netLayer.NewAddr(fallbackStr)

		if err != nil {
			return nil, fmt.Errorf("invalid fallback %v", fallbackStr)
		}

		s.defaultfallbackAddr = fa
	}

	s.addV2User(id)

	return s, nil
}
func (s *Server) CanFallback() bool {
	return true
}
func (s *Server) addV2User(u *proxy.V2rayUser) {
	s.userHashes[*u] = u
}

func (s *Server) AddV2User(u *proxy.V2rayUser) {

	s.mux4Hashes.Lock()
	s.userHashes[*u] = u
	s.mux4Hashes.Unlock()
}

func (s *Server) DelV2User(u *proxy.V2rayUser) {

	s.mux4Hashes.RLock()

	hasu := s.userHashes[*u]
	if hasu == nil {
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
	if s.userHashes[thisUUIDBytes] != nil {
		return proxy.V2rayUser(thisUUIDBytes)
	}
	return nil
}

func (s *Server) HasUserByBytes(bs []byte) bool {
	if len(bs) < 16 {
		return false
	}
	if s.userHashes[*(*[16]byte)(unsafe.Pointer(&bs[0]))] != nil {
		return true
	}
	return false
}

func (s *Server) UserBytesLen() int {
	return 16
}

func (s *Server) GetUserByStr(str string) proxy.User {
	u, e := proxy.StrToUUID(str)
	if e != nil {
		return nil
	}
	return s.GetUserByBytes(u[:])
}

func (s *Server) Name() string { return Name }

// 返回的bytes.Buffer 是用于 回落使用的，内含了整个读取的数据;不回落时不要使用该Buffer
func (s *Server) Handshake(underlay net.Conn) (io.ReadWriter, *netLayer.Addr, error) {

	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		return nil, nil, err
	}
	defer underlay.SetReadDeadline(time.Time{})

	//这里我们本 不用再创建一个buffer来缓存数据，因为tls包本身就是有缓存的，所以一点一点读就行，tcp本身系统也是有缓存的
	// 因此v1.0.3以及更老版本都是直接一段一段read的。
	//但是，因为需要支持fallback技术，所以还是要 进行缓存, 然后返回的UserConn要使用MultiReader，重用之前读过的部分，没办法。

	readbs := utils.GetBytes(utils.StandardBytesLength)

	//var auth [17]byte
	wholeReadLen, err := underlay.Read(readbs)
	if err != nil {
		return nil, nil, utils.NewDataErr("read err", err, wholeReadLen)
	}

	if wholeReadLen < 17 {
		//根据下面回答，HTTP的最小长度恰好是16字节，但是是0.9版本。1.0是18字节，1.1还要更长。总之我们可以直接不返回fallback地址
		//https://stackoverflow.com/questions/25047905/http-request-minimum-size-in-bytes/25065089

		return nil, nil, utils.NewDataErr("fallback, msg too short", nil, wholeReadLen)

	}

	readbuf := bytes.NewBuffer(readbs[:wholeReadLen])

	var returnErr error

	goto realPart

errorPart:

	//fallback 所返回的buffer必须包含所有数据，而Buffer不支持会退，所以只能重新New
	return nil, nil, &httpLayer.ErrSingleFallback{
		FallbackAddr: s.defaultfallbackAddr,
		Err:          returnErr,
		First:        bytes.NewBuffer(readbs[:wholeReadLen]),
	}

realPart:
	//这部分过程可以参照 v2ray的 proxy/vless/encoding/encoding.go DecodeRequestHeader 方法
	//see https://github.com/v2fly/v2ray-core/blob/master/proxy/vless/inbound/inbound.go

	auth := readbuf.Next(17)

	version := auth[0]
	if version > 1 {

		returnErr = utils.NewDataErr("Vless invalid version ", nil, version)
		goto errorPart

	}

	idBytes := auth[1:17]

	s.mux4Hashes.RLock()

	thisUUIDBytes := *(*[16]byte)(unsafe.Pointer(&idBytes[0])) //下面crumfurs也有用到

	if user := s.userHashes[thisUUIDBytes]; user != nil {
		s.mux4Hashes.RUnlock()
	} else {
		s.mux4Hashes.RUnlock()
		returnErr = errors.New("invalid user")
		goto errorPart
	}

	if version == 0 {

		addonLenByte, err := readbuf.ReadByte()
		if err != nil {
			return nil, nil, err //凡是和的层Read相关的错误，一律不再返回Fallback信息，因为连接已然不可用
		}
		if addonLenByte != 0 {
			//v2ray的vless中没有对应的任何处理。
			//v2ray 的 vless 虽然有一个没用的Flow，但是 EncodeBodyAddons里根本没向里写任何数据。所以理论上正常这部分始终应该为0
			if utils.CanLogWarn() {

				log.Println("potential illegal client", addonLenByte)
			}

			//读一下然后直接舍弃
			/*
				tmpBuf := utils.GetBytes(int(addonLenByte))
				underlay.Read(tmpBuf)
				utils.PutBytes(tmpBuf)
			*/
			if tmpbs := readbuf.Next(int(addonLenByte)); len(tmpbs) != int(addonLenByte) {
				return nil, nil, errors.New("vless short read in addon")
			}
		}
	}

	commandByte, err := readbuf.ReadByte()

	if err != nil {

		returnErr = errors.New("fallback, reason 2")
		goto errorPart
	}

	addr := &netLayer.Addr{}

	switch commandByte {
	case proxy.CmdMux: //实际目前暂时verysimple还未实现mux，先这么写

		addr.Port = 0
		addr.Name = "v1.mux.cool"

	case Cmd_CRUMFURS:
		if version != 1 {

			returnErr = errors.New("在vless的vesion不为1时使用了 CRUMFURS 命令")
			goto errorPart

		}

		_, err = underlay.Write([]byte{CRUMFURS_ESTABLISHED})
		if err != nil {

			returnErr = utils.NewErr("write to crumfurs err", err)
			goto errorPart
		}

		addr.Name = CRUMFURS_Established_Str // 使用这个特殊的办法来告诉调用者，预留了 CRUMFURS 信道，防止其关闭上层连接导致 CRUMFURS 信道 被关闭。

		theCRUMFURS := &CRUMFURS{
			Conn: underlay,
		}

		s.mux4Hashes.Lock()

		s.userCRUMFURS[thisUUIDBytes] = theCRUMFURS

		s.mux4Hashes.Unlock()

		return nil, addr, nil

	case proxy.CmdTCP, proxy.CmdUDP:

		portbs := readbuf.Next(2)

		if err != nil || len(portbs) != 2 {

			returnErr = errors.New("fallback, reason 3")
			goto errorPart
		}

		addr.Port = int(binary.BigEndian.Uint16(portbs))

		if commandByte == proxy.CmdUDP {
			addr.IsUDP = true
		}

		var ip_or_domain_bytesLength byte = 0

		addrTypeByte, err := readbuf.ReadByte()

		if err != nil {

			returnErr = errors.New("fallback, reason 4")
			goto errorPart
		}

		switch addrTypeByte {
		case netLayer.AtypIP4:

			ip_or_domain_bytesLength = net.IPv4len
			addr.IP = utils.GetBytes(net.IPv4len)

		case netLayer.AtypDomain:
			// 解码域名的长度

			domainNameLenByte, err := readbuf.ReadByte()

			if err != nil {

				returnErr = errors.New("fallback, reason 5")
				goto errorPart
			}

			ip_or_domain_bytesLength = domainNameLenByte
		case netLayer.AtypIP6:

			ip_or_domain_bytesLength = net.IPv6len
			addr.IP = utils.GetBytes(net.IPv6len)
		default:

			returnErr = fmt.Errorf("unknown address type %v", addrTypeByte)
			goto errorPart
		}

		ip_or_domain := utils.GetBytes(int(ip_or_domain_bytesLength))

		_, err = readbuf.Read(ip_or_domain)

		if err != nil {
			return nil, nil, errors.New("fallback, reason 6")
		}

		if addr.IP != nil {
			copy(addr.IP, ip_or_domain)
		} else {
			addr.Name = string(ip_or_domain)
		}

		utils.PutBytes(ip_or_domain)

	default:

		returnErr = errors.New("invalid vless command")
		goto errorPart
	}

	return &UserConn{
		Conn:           underlay,
		optionalReader: io.MultiReader(readbuf, underlay),
		uuid:           thisUUIDBytes,
		version:        int(version),
		isUDP:          addr.IsUDP,
		isServerEnd:    true,
	}, addr, nil

}
func (s *Server) Stop() {

}

func (s *Server) Get_CRUMFURS(id string) *CRUMFURS {
	bs, err := proxy.StrToUUID(id)
	if err != nil {
		return nil
	}
	return s.userCRUMFURS[bs]
}

type CRUMFURS struct {
	net.Conn
	hasAdvancedLayer bool //在用ws或grpc时，这个开关保持打开
}

func (c *CRUMFURS) WriteUDPResponse(a *net.UDPAddr, b []byte) (err error) {
	atype := netLayer.AtypIP4
	if len(a.IP) > 4 {
		atype = netLayer.AtypIP6
	}
	buf := utils.GetBuf()

	buf.WriteByte(atype)
	buf.Write(a.IP)
	buf.WriteByte(byte(int16(a.Port) >> 8))
	buf.WriteByte(byte(int16(a.Port) << 8 >> 8))

	if !c.hasAdvancedLayer {
		lb := int16(len(b))

		buf.WriteByte(byte(lb >> 8))
		buf.WriteByte(byte(lb << 8 >> 8))
	}
	buf.Write(b)

	_, err = c.Write(buf.Bytes())

	utils.PutBuf(buf)
	return
}
