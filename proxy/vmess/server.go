package vmess

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"golang.org/x/crypto/chacha20poly1305"
)

var ErrReplayAttack = errors.New("vmess: we are under replay attack! ")

var ErrReplaySessionAttack = utils.ErrInErr{ErrDesc: "vmess: duplicated session id, we are under replay attack! ", ErrDetail: ErrReplayAttack}

func init() {
	proxy.RegisterServer(Name, &ServerCreator{})
}

type authPair struct {
	utils.V2rayUser
	cipher.Block
}

func authUserByAuthPairList(bs []byte, authPairList []authPair, antiReplayMachine *authid_antiReplayMachine) (user utils.V2rayUser, err error) {
	now := time.Now().Unix()

	var encrypted_authid [authid_len]byte
	copy(encrypted_authid[:], bs)

	const err_desc = "Vmess AntiReplay Err"

	for _, p := range authPairList {
		failreason := tryMatchAuthIDByBlock(now, p.Block, encrypted_authid, antiReplayMachine)

		switch failreason {

		case 0:
			return p.V2rayUser, nil
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

	}
	return
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
	return s, nil

}

type Server struct {
	proxy.Base

	*utils.MultiUserMap

	authPairList []authPair

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
	b, err := generateCipherByV2rayUser(u)
	if err != nil {
		panic(err)
	}
	p := authPair{
		V2rayUser: u,
		Block:     b,
	}
	s.authPairList = append(s.authPairList, p)
}

func (*Server) HasInnerMux() (int, string) {
	return 1, "simplesocks"
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
	} else if n < authid_len {
		returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 1}
		return
	}
	user, err := authUserByAuthPairList(data[:authid_len], s.authPairList, s.authid_anitReplayMachine)
	if err != nil {

		returnErr = err
		return

	}

	cmdKey := GetKey(user)
	remainBuf := bytes.NewBuffer(data[authid_len:n])

	aeadData, shouldDrain, bytesRead, errorReason := openAEADHeader(cmdKey, data[:16], remainBuf)
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
	if len(aeadData) < 38 {
		returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 3}
		return
	}

	//https://www.v2fly.org/developer/protocols/vmess.html#%E6%8C%87%E4%BB%A4%E9%83%A8%E5%88%86
	sc := &ServerConn{
		version:   int(aeadData[0]),
		Conn:      underlay,
		V2rayUser: user,
		reqRespV:  aeadData[33],
		opt:       aeadData[34],
		security:  aeadData[35] & 0x0f,
		cmd:       aeadData[37],
	}

	copy(sc.reqBodyIV[:], aeadData[1:17])
	copy(sc.reqBodyKey[:], aeadData[17:33])

	paddingLen := int(aeadData[35] >> 4)

	lenBefore := len(aeadData[38:])

	aeadDataBuf := bytes.NewBuffer(aeadData[38:])

	var sid sessionID
	copy(sid.user[:], user.IdentityBytes())
	sid.key = sc.reqBodyKey
	sid.nonce = sc.reqBodyIV

	if !s.session_antiReplayMachine.check(sid) {
		returnErr = ErrReplaySessionAttack
		return
	}

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

	if paddingLen > 0 {
		tmpBs := aeadDataBuf.Next(paddingLen)
		if len(tmpBs) != paddingLen {
			returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 5}
			return
		}
	}

	lenAfter := aeadDataBuf.Len()
	realLen := lenBefore - lenAfter + 38

	fnv1a := fnv.New32a()
	fnv1a.Write(aeadData[:realLen])
	actualHash := fnv1a.Sum32()

	expectedHash := binary.BigEndian.Uint32(aeadDataBuf.Next(4))

	if actualHash != expectedHash {
		returnErr = utils.NumErr{E: utils.ErrInvalidData, N: 6}
		return
	}

	sc.remainReadBuf = remainBuf

	buf := utils.GetBuf()

	sc.aead_encodeRespHeader(buf)
	sc.firstWriteBuf = buf

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
	version  int
	opt      byte
	security byte
	cmd      byte
	reqRespV byte

	theTarget netLayer.Addr

	reqBodyIV   [16]byte
	reqBodyKey  [16]byte
	respBodyIV  [16]byte
	respBodyKey [16]byte

	remainReadBuf, firstWriteBuf *bytes.Buffer

	dataReader io.Reader
	dataWriter io.Writer
}

func (s *ServerConn) aead_encodeRespHeader(outBuf *bytes.Buffer) error {
	BodyKey := sha256.Sum256(s.reqBodyKey[:])
	copy(s.respBodyKey[:], BodyKey[:16])
	BodyIV := sha256.Sum256(s.reqBodyIV[:])
	copy(s.respBodyIV[:], BodyIV[:16])

	encryptionWriter := utils.GetBuf()
	encryptionWriter.Write([]byte{s.reqRespV, 0})
	encryptionWriter.Write([]byte{0x00, 0x00}) //我们暂时不支持动态端口，太复杂, 懒。

	aeadResponseHeaderLengthEncryptionKey := kdf16(s.respBodyKey[:], kdfSaltConstAEADRespHeaderLenKey)
	aeadResponseHeaderLengthEncryptionIV := kdf(s.respBodyIV[:], kdfSaltConstAEADRespHeaderLenIV)[:12]

	aeadResponseHeaderLengthEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderLengthEncryptionKey)
	aeadResponseHeaderLengthEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderLengthEncryptionKeyAESBlock)

	aeadResponseHeaderLengthEncryptionBuffer := bytes.NewBuffer(nil)

	decryptedResponseHeaderLengthBinaryDeserializeBuffer := uint16(encryptionWriter.Len())

	binary.Write(aeadResponseHeaderLengthEncryptionBuffer, binary.BigEndian, decryptedResponseHeaderLengthBinaryDeserializeBuffer)

	AEADEncryptedLength := aeadResponseHeaderLengthEncryptionAEAD.Seal(nil, aeadResponseHeaderLengthEncryptionIV, aeadResponseHeaderLengthEncryptionBuffer.Bytes(), nil)
	io.Copy(outBuf, bytes.NewReader(AEADEncryptedLength))

	aeadResponseHeaderPayloadEncryptionKey := kdf16(s.respBodyKey[:], kdfSaltConstAEADRespHeaderPayloadKey)
	aeadResponseHeaderPayloadEncryptionIV := kdf(s.respBodyIV[:], kdfSaltConstAEADRespHeaderPayloadIV)[:12]

	aeadResponseHeaderPayloadEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderPayloadEncryptionKey)
	aeadResponseHeaderPayloadEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderPayloadEncryptionKeyAESBlock)

	aeadEncryptedHeaderPayload := aeadResponseHeaderPayloadEncryptionAEAD.Seal(nil, aeadResponseHeaderPayloadEncryptionIV, encryptionWriter.Bytes(), nil)

	io.Copy(outBuf, bytes.NewReader(aeadEncryptedHeaderPayload))
	return nil

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

	var shakeParser *ShakeSizeParser

	if c.opt&OptChunkMasking == OptChunkMasking {

		shouldPad := false
		if c.opt&OptGlobalPadding == OptGlobalPadding {
			shouldPad = true

		}
		shakeParser = NewShakeSizeParser(c.respBodyIV[:], shouldPad)
	}

	if c.opt&OptChunkStream == OptChunkStream {
		switch c.security {
		case SecurityNone:
			c.dataWriter = ChunkedWriter(writer)

		case SecurityAES128GCM:
			block, _ := aes.NewCipher(c.respBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataWriter = AEADWriter(writer, aead, c.respBodyIV[:], shakeParser)

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.respBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataWriter = AEADWriter(writer, aead, c.respBodyIV[:], shakeParser)
			utils.PutBytes(key)
		}
	}

	n, err = c.dataWriter.Write(b)

	close(switchChan)

	if err != nil {
		return
	}
	_, err = c.Conn.Write(c.firstWriteBuf.Bytes())
	utils.PutBuf(c.firstWriteBuf)
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
	var shakeParser *ShakeSizeParser

	if c.opt&OptChunkMasking > 0 {

		shouldPad := false

		if c.opt&OptGlobalPadding > 0 {
			shouldPad = true

		}

		shakeParser = NewShakeSizeParser(c.reqBodyIV[:], shouldPad)
	}

	if c.opt&OptChunkStream > 0 {
		switch c.security {
		case SecurityNone:
			c.dataReader = ChunkedReader(curReader)

		case SecurityAES128GCM:

			block, _ := aes.NewCipher(c.reqBodyKey[:])
			aead, _ := cipher.NewGCM(block)
			c.dataReader = AEADReader(curReader, aead, c.reqBodyIV[:], shakeParser)

		case SecurityChacha20Poly1305:
			key := utils.GetBytes(32)
			t := md5.Sum(c.reqBodyKey[:])
			copy(key, t[:])
			t = md5.Sum(key[:16])
			copy(key[16:], t[:])
			aead, _ := chacha20poly1305.New(key)
			c.dataReader = AEADReader(curReader, aead, c.reqBodyIV[:], shakeParser)
			utils.PutBytes(key)
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
