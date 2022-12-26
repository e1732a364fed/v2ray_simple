package vmess

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"net"
	"net/url"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/cloudflare/circl/kem"
	"github.com/cloudflare/circl/kem/mceliece/mceliece8192128f"
)

var ErrReplayAttack = errors.New("vmess: we are under replay attack! ")

var ErrReplaySessionAttack = utils.ErrInErr{ErrDesc: "vmess: duplicated session id, we are under replay attack! ", ErrDetail: ErrReplayAttack}

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type ServerCreator struct{ proxy.CreatorCommonStruct }

func (ServerCreator) URLToListenConf(url *url.URL, lc *proxy.ListenConf, format int) (*proxy.ListenConf, error) {

	switch format {
	case proxy.UrlStandardFormat:
		if lc == nil {
			lc = &proxy.ListenConf{}

			uuidStr := url.User.Username()
			lc.Uuid = uuidStr
		}

		return lc, nil
	default:
		return lc, utils.ErrUnImplemented
	}

}

func (ServerCreator) NewServer(lc *proxy.ListenConf) (proxy.Server, error) {
	uuidStr := lc.Uuid

	s := NewServer()

	if uuidStr != "" {
		v2rayUser, err := utils.NewV2rayUser(uuidStr)
		if err != nil {
			return nil, err
		}
		s.addUser(v2rayUser)
	}

	if len(lc.Users) > 0 {
		us := utils.InitRealV2rayUsers(lc.Users)
		for _, u := range us {
			s.addUser(u)
		}
	}
	if len(lc.Extra) > 0 {
		if thing := lc.Extra["server_privatekey"]; thing != nil {
			if str, ok := thing.(string); ok {
				ds, err := hex.DecodeString(str)
				if err != nil {
					return nil, err
				}
				pri, err := mceliece8192128f.Scheme().UnmarshalBinaryPrivateKey(ds)
				if err != nil {
					return nil, err
				}
				s.srvpri = pri
			}
		}
	}
	return s, nil

}

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	UserList []utils.V2rayUser

	srvpri kem.PrivateKey

	authid_anitReplayMachine  *authid_antiReplayMachine
	session_antiReplayMachine *session_antiReplayMachine
}

func NewServer() *Server {
	s := &Server{
		MultiUserMap:              utils.NewMultiUserMap(),
		authid_anitReplayMachine:  newAuthIDAntiReplyMachine(),
		session_antiReplayMachine: newSessionAntiReplayMachine(),
	}
	s.SetUseUUIDStr_asKey()
	return s
}
func (s *Server) Name() string { return Name }

func (s *Server) Stop() {
	s.authid_anitReplayMachine.stop()
	s.session_antiReplayMachine.stop()
}

func (s *Server) addUser(u utils.V2rayUser) {
	s.MultiUserMap.AddUser_nolock(u)
	s.UserList = append(s.UserList, u)
}

func (*Server) HasInnerMux() (int, string) {
	return 1, "simplesocks"
}

type msession struct {
	sharedsecret []byte
	user         utils.V2rayUser
}

func (s *Server) authUserByUserList(bs []byte, UserList []utils.V2rayUser, antiReplayMachine *authid_antiReplayMachine) (session msession, err error) {
	encap := bs[:mceliece8192128f.CiphertextSize]
	const err_desc = "Vmess AntiReplay Err"
	ss, err := mceliece8192128f.Scheme().Decapsulate(s.srvpri, encap)
	if err != nil {
		return
	}
	var t int64

	timekey := kdf(ss, []byte(kdfSaltConstAEADKEY), []byte("time"))

	aead, err := chacha20poly1305.New(timekey[:chacha20poly1305.KeySize])
	if err != nil {
		return
	}
	dectime, err := aead.Open(nil, timekey[chacha20poly1305.KeySize:chacha20poly1305.KeySize+chacha20poly1305.NonceSize], bs[mceliece8192128f.CiphertextSize:], nil)
	if err != nil {
		return
	}
	buf := bytes.NewBuffer(dectime)
	binary.Read(buf, binary.BigEndian, &t)
	now := time.Now().Unix()
	for _, p := range UserList {
		failreason := tryMatchAuthIDByBlock(now, t, antiReplayMachine)

		switch failreason {
		case 0:
			return msession{user: p, sharedsecret: ss}, nil
		case 1: //crc
			err = utils.ErrInErr{ErrDesc: err_desc, ErrDetail: utils.ErrInvalidData}
			//crc校验失败只是证明是随机数据 或者是当前uuid不匹配，需要继续匹配。
		case 2:
			err = utils.ErrInErr{ErrDesc: err_desc, ErrDetail: ErrAuthID_timeBeyondGap}
		case 3:
			err = utils.ErrInErr{ErrDesc: err_desc, ErrDetail: ErrReplayAttack}
		}
	}
	if err == nil {
		err = utils.ErrNoMatch
		panic(err)
	}
	return
}

// 为0表示匹配成功, 如果不为0，则匹配失败；
// 若为1，则CRC 校验失败（正常地匹配失败，不意味着被攻击）; 若为2，则表明校验成功 但是 时间差距超过 authID_timeMaxSecondGap 秒，如果为3，则表明遇到了重放攻击。
func tryMatchAuthIDByBlock(now int64, t int64, anitReplayMachine *authid_antiReplayMachine) (failReason int) {

	if math.Abs(math.Abs(float64(t))-float64(now)) > authID_timeMaxSecondGap {
		return 2
	}

	return 0
}

func (s *Server) Handshake(underlay net.Conn) (tcpConn net.Conn, msgConn netLayer.MsgConn, targetAddr netLayer.Addr, returnErr error) {
	if err := netLayer.SetCommonReadTimeout(underlay); err != nil {
		returnErr = err
		return
	}
	defer netLayer.PersistConn(underlay)

	data := utils.GetPacket()

	n, err := underlay.Read(data)
	if err != nil {
		returnErr = err
		return
	} else if n < mceliece8192128f.Scheme().CiphertextSize()+8+chacha20poly1305.Overhead {
		returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 1}
		return
	}
	session, err := s.authUserByUserList(data[:mceliece8192128f.CiphertextSize+8+chacha20poly1305.Overhead], s.UserList, s.authid_anitReplayMachine)
	if err != nil {

		returnErr = err
		return

	}

	remainBuf := bytes.NewBuffer(data[mceliece8192128f.CiphertextSize+8+chacha20poly1305.Overhead : n])

	aeadData, shouldDrain, bytesRead, errorReason := openAEADHeader(session.sharedsecret, remainBuf)
	if errorReason != nil {
		returnErr = errorReason
		if ce := utils.CanLogWarn("vmess openAEADHeader err"); ce != nil {
			//v2ray代码中有一个 "drain"的用法，
			//然而，我们这里是不需要drain的，区别在于，v2ray 不是一次性读取一大串数据，
			// 而是用一个 reader 一点一点读，这就会产生一些可探测的问题，所以才要drain
			// 而我们直接用 64K 的大buf 一下子读取整个客户端发来的整个数据， 没有读取长度的差别。
			//不过 为了尊重v2ray的代码，也 以防 我的想法有错误，还是把这个情况陈列在这里，留作备用。
			ce.Write(zap.Any("things", []any{errorReason, shouldDrain, bytesRead}))
		}

		return
	}

	if len(aeadData) < 3 {
		returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 3}
		return
	}

	//https://www.v2fly.org/developer/protocols/vmess.html#%E6%8C%87%E4%BB%A4%E9%83%A8%E5%88%86
	sc := &ServerConn{
		Conn:         underlay,
		V2rayUser:    session.user,
		opt:          aeadData[0],
		security:     aeadData[1],
		cmd:          aeadData[2],
		sharedsecret: session.sharedsecret,
	}

	aeadDataBuf := bytes.NewBuffer(aeadData[3:])

	var sid sessionID
	copy(sid.user[:], session.user[:])

	// if !s.session_antiReplayMachine.check(session.user[:]) {
	// 	returnErr = ErrReplaySessionAttack
	// 	return
	// }

	var ismux bool

	switch sc.cmd {
	case CmdTCP, CmdUDP:
		ad, err := netLayer.V2rayGetAddrFrom(aeadDataBuf)
		if err != nil {
			returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 4}
			return
		}
		sc.theTarget = ad
		if sc.cmd == CmdUDP {
			ad.Network = "udp"
		}
		targetAddr = ad

		//verysimple 不支持v2ray中的 vmess 的 mux.cool

	case cmd_muxcool_unimplemented:
		returnErr = utils.ErrInErr{ErrDesc: "Vmess mux.cool is not supported by verysimple ", ErrDetail: utils.ErrInvalidData}

	case CMDMux_VS:
		ismux = true

		_, err := netLayer.V2rayGetAddrFrom(aeadDataBuf)
		if err != nil {
			returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 4}
			return
		}

	default:

		returnErr = utils.ErrInErr{ErrDesc: "Vmess Invalid command ", ErrDetail: utils.ErrInvalidData, Data: sc.cmd}

		return
	}

	sc.remainReadBuf = remainBuf
	// buf := utils.GetBuf()

	// sc.aead_encodeRespHeader(buf)
	// sc.firstWriteBuf = buf
	sc.firstWriteBuf = nil
	if ismux {

		mh := &proxy.MuxMarkerConn{
			ReadWrapper: netLayer.ReadWrapper{
				Conn: sc,
			},
		}

		if l := remainBuf.Len(); l > 0 {
			mh.RemainFirstBufLen = l
			mh.OptionalReader = io.MultiReader(remainBuf, underlay)
		}

		return mh, nil, targetAddr, nil
	}

	if sc.cmd == CmdTCP {
		tcpConn = sc

	} else {
		msgConn = sc
	}

	return
}

type ServerConn struct {
	net.Conn

	utils.V2rayUser
	opt      byte
	security byte
	cmd      byte

	theTarget netLayer.Addr

	sharedsecret []byte

	remainReadBuf, firstWriteBuf *bytes.Buffer

	dataReader io.Reader
	dataWriter io.Writer
}

func (c *ServerConn) Write(b []byte) (n int, err error) {

	if c.dataWriter != nil {
		return c.dataWriter.Write(b)
	}
	switchChan := make(chan struct{})

	//使用 utils.WriteSwitcher 来 粘连 服务器vmess响应 以及第一个数据响应
	writer := &utils.WriteSwitcher{
		Old:        c.firstWriteBuf,
		New:        c.Conn,
		SwitchChan: switchChan,
		Closer:     c.Conn,
	}

	c.dataWriter = writer

	if c.opt&OptChunkStream == OptChunkStream {
		s2c := kdf(c.sharedsecret, []byte(kdfSaltConstAEADKEY), []byte("Server2Client"))
		switch c.security {
		case SecurityNone:
			c.dataWriter = ChunkedWriter(writer)
		case SecurityAES256GCM:
			block, _ := aes.NewCipher(s2c[:chacha20poly1305.KeySize])
			aead, _ := cipher.NewGCM(block)
			c.dataWriter = AEADWriter(writer, aead, s2c[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		case SecurityChacha20Poly1305:
			aead, _ := chacha20poly1305.New(s2c[:chacha20poly1305.KeySize])
			c.dataWriter = AEADWriter(writer, aead, s2c[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		}
	}

	n, err = c.dataWriter.Write(b)
	if err != nil {
		panic(err)
	}
	close(switchChan)

	if err != nil {
		return
	}
	_, err = c.Conn.Write(c.firstWriteBuf.Bytes())
	defer utils.PutBuf(c.firstWriteBuf)
	c.firstWriteBuf = nil
	return
}

func (c *ServerConn) Read(b []byte) (n int, err error) {

	if c.dataReader != nil {
		return c.dataReader.Read(b)
	}
	var curReader io.Reader
	if c.remainReadBuf != nil && c.remainReadBuf.Len() > 0 {
		curReader = io.MultiReader(c.remainReadBuf, c.Conn)
	} else {
		curReader = c.Conn

	}

	if c.opt&OptChunkStream > 0 {
		c2s := kdf(c.sharedsecret, []byte(kdfSaltConstAEADKEY), []byte("Client2Server"))
		switch c.security {
		case SecurityNone:
			c.dataReader = ChunkedReader(curReader)
		case SecurityAES256GCM:
			block, _ := aes.NewCipher(c2s[:chacha20poly1305.KeySize])
			aead, _ := cipher.NewGCM(block)
			c.dataReader = AEADReader(curReader, aead, c2s[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])

		case SecurityChacha20Poly1305:
			aead, _ := chacha20poly1305.New(c2s[:chacha20poly1305.KeySize])
			c.dataReader = AEADReader(curReader, aead, c2s[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()])
		}
	}

	if c.dataReader == nil {
		//c.opt == OptBasicFormat (0) 时即出现此情况
		return 0, utils.ErrInErr{ErrDesc: "vmess server might get an old vmess client, closing. (c.dataReader==nil)", Data: c.opt}
	}

	return c.dataReader.Read(b)

}

func (c *ServerConn) ReadMsg() (bs []byte, target netLayer.Addr, err error) {
	bs = utils.GetPacket()
	var n int
	n, err = c.Read(bs)
	if err != nil {
		utils.PutPacket(bs)
		bs = nil
		return
	}
	bs = bs[:n]
	target = c.theTarget
	return
}

func (c *ServerConn) WriteMsg(b []byte, _ netLayer.Addr) error {
	_, e := c.Write(b)
	return e
}
func (c *ServerConn) CloseConnWithRaddr(_ netLayer.Addr) error {
	return c.Conn.Close()
}
func (c *ServerConn) Fullcone() bool {
	return false
}
