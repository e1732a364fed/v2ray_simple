package vless

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/hahahrfool/v2ray_simple/common"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func init() {
	proxy.RegisterServer(Name, NewVlessServer)
}

//实现 proxy.UserServer 以及 tlsLayer.UserHaser
type Server struct {
	proxy.ProxyCommonStruct
	userHashes   map[[16]byte]*proxy.V2rayUser
	userCRUMFURS map[[16]byte]*CRUMFURS
	mux4Hashes   sync.RWMutex
}

func NewVlessServer(url *url.URL) (proxy.Server, error) {

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

	s.addV2User(id)

	return s, nil
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

//see https://github.com/v2fly/v2ray-core/blob/master/proxy/vless/inbound/inbound.go
func (s *Server) Handshake(underlay net.Conn) (io.ReadWriter, *proxy.Addr, error) {

	if err := underlay.SetReadDeadline(time.Now().Add(time.Second * 4)); err != nil {
		return nil, nil, err
	}
	defer underlay.SetReadDeadline(time.Time{})

	var auth [17]byte
	num, err := underlay.Read(auth[:])
	if err != nil {
		return nil, nil, common.NewDataErr("read err", err, auth)
	}

	if num < 17 {
		return nil, nil, common.NewDataErr("fallback, msg too short", nil, num)

	}

	//这部分过程可以参照 v2ray的 proxy/vless/encoding/encoding.go DecodeRequestHeader 方法

	version := auth[0]
	if version > 1 {
		return nil, nil, errors.New("Vless invalid request version " + strconv.Itoa(int(version)))
	}

	idBytes := auth[1:17]

	s.mux4Hashes.RLock()

	thisUUIDBytes := *(*[16]byte)(unsafe.Pointer(&idBytes[0]))

	if user := s.userHashes[thisUUIDBytes]; user != nil {
		s.mux4Hashes.RUnlock()
	} else {
		s.mux4Hashes.RUnlock()
		return nil, nil, errors.New("invalid user")
	}

	if version == 0 {
		var addonLenBytes [1]byte
		_, err := underlay.Read(addonLenBytes[:])
		if err != nil {
			return nil, nil, err
		}
		addonLenByte := addonLenBytes[0]
		if addonLenByte != 0 {
			//v2ray的vless中没有对应的任何处理。
			//v2ray 的 vless 虽然有一个没用的Flow，但是 EncodeBodyAddons里根本没向里写任何数据。所以理论上正常这部分始终应该为0
			log.Println("potential illegal client", addonLenByte)

			//读一下然后直接舍弃
			tmpBuf := common.GetBytes(int(addonLenByte))
			underlay.Read(tmpBuf)
			common.PutBytes(tmpBuf)
		}
	}

	var commandBytes [1]byte
	num, err = underlay.Read(commandBytes[:])

	if err != nil {
		return nil, nil, errors.New("fallback, reason 2")
	}

	commandByte := commandBytes[0]

	addr := &proxy.Addr{}

	switch commandByte {
	case proxy.CmdMux: //实际目前暂时v2simple还未实现mux，先这么写

		addr.Port = 0
		addr.Name = "v1.mux.cool"

	case Cmd_CRUMFURS:
		if version != 1 {
			return nil, nil, errors.New("在vless的vesion不为1时使用了 CRUMFURS 命令")
		}

		_, err = underlay.Write([]byte{CRUMFURS_ESTABLISHED})
		if err != nil {
			return nil, nil, err
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

		var portbs [2]byte

		num, err = underlay.Read(portbs[:])

		if err != nil || num != 2 {
			return nil, nil, errors.New("fallback, reason 3")
		}

		addr.Port = int(binary.BigEndian.Uint16(portbs[:]))

		if commandByte == proxy.CmdUDP {
			addr.IsUDP = true
		}

		var ip_or_domain_bytesLength byte = 0

		var addrTypeBytes [1]byte
		_, err = underlay.Read(addrTypeBytes[:])

		if err != nil {
			return nil, nil, errors.New("fallback, reason 4")
		}

		addrTypeByte := addrTypeBytes[0]

		switch addrTypeByte {
		case proxy.AtypIP4:

			ip_or_domain_bytesLength = net.IPv4len
			addr.IP = common.GetBytes(net.IPv4len)
		case proxy.AtypDomain:
			// 解码域名的长度

			var domainNameLenBytes [1]byte
			_, err = underlay.Read(domainNameLenBytes[:])

			if err != nil {
				return nil, nil, errors.New("fallback, reason 5")
			}

			domainNameLenByte := domainNameLenBytes[0]

			ip_or_domain_bytesLength = domainNameLenByte
		case proxy.AtypIP6:

			ip_or_domain_bytesLength = net.IPv6len
			addr.IP = common.GetBytes(net.IPv6len)
		default:
			return nil, nil, fmt.Errorf("unknown address type %v", addrTypeByte)
		}

		ip_or_domain := common.GetBytes(int(ip_or_domain_bytesLength))

		_, err = underlay.Read(ip_or_domain[:])

		if err != nil {
			return nil, nil, errors.New("fallback, reason 6")
		}

		if addr.IP != nil {
			copy(addr.IP, ip_or_domain)
		} else {
			addr.Name = string(ip_or_domain)
		}

		common.PutBytes(ip_or_domain)

	default:
		return nil, nil, errors.New("invalid vless command")
	}

	return &UserConn{
		Conn:        underlay,
		uuid:        thisUUIDBytes,
		version:     int(version),
		isUDP:       addr.IsUDP,
		isServerEnd: true,
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
	atype := proxy.AtypIP4
	if len(a.IP) > 4 {
		atype = proxy.AtypIP6
	}
	buf := common.GetBuf()

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

	common.PutBuf(buf)
	return
}
