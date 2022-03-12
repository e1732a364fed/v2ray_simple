package tlsLayer

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net"
	"strings"
)

var PDD bool //print tls detect detail
var OnlyTest bool

func init() {
	//log.SetOutput(os.Stdout) //主要是日志太多，如果都能直接用管道放到文件中就好了，默认不是Stdout所以优点尴尬，操作麻烦点

	flag.BoolVar(&PDD, "pdd", false, "print tls detect detail")
	flag.BoolVar(&OnlyTest, "ot", false, "only detect tls, doesn't actually mark tls")

}

// 用于 探测 承载数据是否使用了tls, 它先与 底层tcp连接 进行 数据传输，然后查看传输到内容
// 	可以参考 https://www.baeldung.com/linux/tcpdump-capture-ssl-handshake
type DetectConn struct {
	net.Conn //这个 Conn本DetectConn 中不会用到，只是为了能让CopyConn支持 net.Conn
	W        *DetectWriter
	R        *DetectReader

	RawConn *net.TCPConn // 这个是为了让外界能够直接拿到底层的连接

}

func (cc *DetectConn) Read(p []byte) (int, error) {
	return cc.R.Read(p)
}

func (cc *DetectConn) Write(p []byte) (int, error) {
	return cc.W.Write(p)
}

//这个暂时没用到，先留着
func (cc *DetectConn) ReadFrom(r io.Reader) (int64, error) {
	if cc.RawConn != nil {
		return cc.RawConn.ReadFrom(r)
	}
	return 0, io.EOF
}

//可选两个参数传入，优先使用rw ，为nil的话 再使用oldConn，作为 DetectConn 的 Read 和Write的 具体调用的主体
func NewDetectConn(oldConn net.Conn, rw io.ReadWriter, isclient bool) *DetectConn {

	var validOne io.ReadWriter = rw
	if rw == nil {
		validOne = oldConn
	}

	cc := &DetectConn{
		Conn: oldConn,
		W: &DetectWriter{
			Writer: validOne,
		},
		R: &DetectReader{
			Reader: validOne,
		},
	}
	cc.W.isclient = isclient
	cc.R.isclient = isclient

	if netConn := oldConn.(*net.TCPConn); netConn != nil {
		//log.Println("get netConn!")	// 如果是客户端的socks5，网页浏览的话这里一定能转成 TCPConn
		cc.RawConn = netConn
	}

	return cc
}

type ComDetectStruct struct {
	IsTls            bool
	DefinitelyNotTLS bool

	isclient bool

	packetCount int

	handShakePass bool

	handshakeFailReason int
}

const (
	CutType_big byte = iota + 1
	CutType_small
	CutType_fit
)

func (c *ComDetectStruct) incr() {
	c.packetCount++
}

func (c *ComDetectStruct) GetFailReason() int {
	return c.handshakeFailReason
}

// 总之，如果读写都用同样的判断代码的话，客户端和服务端应该就能同步进行 相同的TLS判断
func commonDetect(cd *ComDetectStruct, p []byte, isRead bool) {

	/*
		我们把tls的细节放在这个注释里，便于参考

		首先判断握手包，即第一个包
		The Client Hello messages contain 01 in the sixth data byte of the TCP packet.
		应该是这里定义的： https://datatracker.ietf.org/doc/html/rfc5246#section-7.4

		//不过，首先要包在 Plaintex结构里
		//https://datatracker.ietf.org/doc/html/rfc5246

		struct {
				ContentType type; //第一字节
				ProtocolVersion version;//第2、3字节
				uint16 length;// 第4、5字节
				opaque fragment[TLSPlaintext.length];
			} TLSPlaintext;

		//ContentType 中，handshake始终是22，也就是说，tls连接的hello字节第一个肯定是22
		// 从tls1.0 到 1.3， 这个情况都是一样的

		tls1.0, tls1.2:  change_cipher_spec(20), alert(21), handshake(22),application_data(23)

		 	enum {
				hello_request(0), client_hello(1), server_hello(2),
				certificate(11), server_key_exchange (12),
				certificate_request(13), server_hello_done(14),
				certificate_verify(15), client_key_exchange(16),
				finished(20), (255)
			} HandshakeType;

			struct {
				HandshakeType msg_type;    // handshake type，第六字节
				uint24 length;             // bytes in message，第789字节
				select (HandshakeType) {
					case hello_request:       HelloRequest;
					case client_hello:        ClientHello;
					case server_hello:        ServerHello;
					case certificate:         Certificate;
					case server_key_exchange: ServerKeyExchange;
					case certificate_request: CertificateRequest;
					case server_hello_done:   ServerHelloDone;
					case certificate_verify:  CertificateVerify;
					case client_key_exchange: ClientKeyExchange;
					case finished:            Finished;
				} body;
			} Handshake;

			然后见下面 TLSPlaintext 的定义，前五字节是 头部, 然后第六字节就是这个Handshake的第一字节，client就是 client_hello, 就是1

			In the SSL handshake message, the 10th and 11th bytes of the data contain the TLS version.

			数一下，ClientHello结构和 ServerHello结构 正好在第十字节的位置 开始

			struct {
				ProtocolVersion client_version;
				Random random;
				SessionID session_id;
				CipherSuite cipher_suites<2..2^16-2>;
				CompressionMethod compression_methods<1..2^8-1>;
				select (extensions_present) {
				case false:
					struct {};
				case true:
					Extension extensions<0..2^16-1>;
				};
			} ClientHello;

			ProtocolVersion:
			TLSv1.0 – 0x0301
			TLSv1.1 – 0x0302
			TLSv1.2 – 0x0303
			TLSv1.3 – 0x0304

		数据部分：

		tls1.2
		https://datatracker.ietf.org/doc/html/rfc5246#section-6.2

		struct {
			uint8 major;
			uint8 minor;
		} ProtocolVersion;

		enum {
			change_cipher_spec(20), alert(21), handshake(22),
			application_data(23), (255)
		} ContentType;

		struct {
			ContentType type;
			ProtocolVersion version;
			uint16 length;
			opaque fragment[TLSPlaintext.length];
		} TLSPlaintext;

		struct {
			ContentType type;       //23
			ProtocolVersion version;	//3,3
			uint16 length;				//第4，5字节
			opaque fragment[TLSCompressed.length];
		} TLSCompressed;

	*/

	n := len(p)

	defer cd.incr()

	p0 := p[0]

	if cd.packetCount == 0 {

		if n < 11 {
			cd.handshakeFailReason = 1
			return
		}
		if p0 != 22 {
			cd.handshakeFailReason = 2

			return
		}

		//version: p9, p10 , 3,1 3,2 3,3 3,4
		if p[9] != 3 {
			cd.handshakeFailReason = 4
			return
		}
		if p[10] == 0 || p[10] > 4 {
			cd.handshakeFailReason = 5
			return
		}

		var helloValue byte = 1

		//我们 DetectConn中，考察客户端传输来的流量 时 使用 Read， 考察服务端发来的流量 时 使用Write
		if !isRead {
			helloValue = 2
		}

		if p[5] != helloValue { //第六字节，
			cd.handshakeFailReason = 3
			return
		}

		cd.handShakePass = true

	} else if !cd.handShakePass {
		cd.DefinitelyNotTLS = true
		return
	}

	if !OnlyTest {
		if !isRead {
			// 如果前面握手符合 tls特征，然后之后的命令却获得到了特殊命令的头部，则我们直接认为这是服务端发给我们的特殊指令

			// 这个过程依然是 在 客户端的 读取出 wrc的数据，然后 调用 wlcdc.Write 时发生
			// 就是说，同样是Write，服务端是在Write判断完后，发送特殊指令
			// 而客户端是在 Write的判断过程中，检索特殊指令
			// 说白了，就是在 服务端-> 客户端 这个方向发生的，
			if cd.isclient && n >= len(SpecialCommand) {

				if bytes.Equal(SpecialCommand, p[:len(SpecialCommand)]) {

					if PDD {
						log.Println("W 读到特殊命令！")
					}

					cd.IsTls = true
					return
				}
			}
		} else {
			// Read也是同理，也是发送特殊指令，只不过Read的话，是客户端向 服务端 发送特殊指令
			// 这里是不会被黑客攻击的，因为事件发生在第二个或者更往后的 数据包中，而vless的uuid检验则是从第一个就要开始检验。

			//这里就是服务端来读取 特殊指令
			if !cd.isclient && n >= len(SpecialCommand) {

				if PDD {
					log.Println("R 尝试读取特殊指令")
				}

				if bytes.Equal(SpecialCommand, p[:len(SpecialCommand)]) {
					if PDD {
						log.Println("R 读到特殊命令！")
					}
					cd.IsTls = true
					return
				}
			} else {

			}
		}
	}

	//下面的判断， 只会在 客户端的 Read，和 服务端的 Write中 发生

	p1 := p[1]
	p2 := p[2]

	//客户端Read 时 少数情况 第一字节21，首部为  [21 3 3 0 26 0 0 0 0 0], 一般总长度为31
	//	不过后来发现，这就是xtls里面隔离出的 alert的那种情况，
	// 其它都是 能被捕捉到的。

	// 23表示 数据层，第二字节3是固定的，然后第3字节 从1到4，表示tls版本，不过tls1.3 和 1.2这里都是 23 3 3
	if p0 == 23 && p1 == 3 && p2 > 0 && p2 < 4 {
		if PDD {
			str := "W"
			if isRead {
				str = "R"
			}
			log.Println(str, "got TLS!", n)
		}

		if !OnlyTest {

			cd.IsTls = true
		}

		return
	}

	// 打印没过滤到的数据，一般而言，大多是非首包的 握手数据，以22开头
	if PDD || OnlyTest {
		min := 10
		if n < 10 {
			min = n
		}

		str := "Write,"
		if isRead {
			str = "Read,"
		}
		log.Println(" ======== ", str, n, p[:min])
	}

}

func commonFilterStep(err error, cd *ComDetectStruct, p []byte, isRead bool) {
	if !OnlyTest && (cd.DefinitelyNotTLS || cd.IsTls) { //确定了是TLS 或者肯定不是 tls了的话，就直接return掉
		return
	}
	if err != nil {
		eStr := err.Error()
		if strings.Contains(eStr, "use of closed") || strings.Contains(eStr, "reset by peer") || strings.Contains(eStr, "EOF") {
			return
		}
	}
	if !OnlyTest && cd.packetCount > 8 {
		//都8个包了还没断定tls？直接推定不是！
		cd.handshakeFailReason = -1
		cd.DefinitelyNotTLS = true
		return
	}

	if len(p) > 3 {
		commonDetect(cd, p, isRead)
	}

}

// DetectReader 对每个Read的数据进行分析，判断是否是tls流量
type DetectReader struct {
	io.Reader

	ComDetectStruct
}

// 总之，我们在客户端的 Read 操作，就是 我们试图使用 Read 读取客户的请求，然后试图发往 外界
//  所以在socks5后面 使用的这个 Read，是读取客户端发送的请求，比如 clienthello之类
//	服务端的 Read 操作，也是读 clienthello，因为我们总是判断客户传来的数据
func (dr *DetectReader) Read(p []byte) (n int, err error) {
	n, err = dr.Reader.Read(p)

	commonFilterStep(err, &dr.ComDetectStruct, p[:n], true)

	/*
		if dr.IsTls {

			if dr.isclient {
				//判断TLS成功后，会向服务端发送特殊指令

				//特殊指令是直接在main.go里发送的, 即调用Read者 负责发送特殊指令

				// 此时特殊指令的发送不受Reader控制，要在main函数中 Write 到 wrc中
				return
			} else {
				// 服务端，能进入这里，就代表服务端接收到了特殊指令

				//那么此时的p的前几字节都是特殊指令，是没用的，要跳过；后面是可能会接东西的，也可能不接，因为tcp是粘包的
				//后面如果接了任何东西，那么接的东西都是需要 直连发送的

				// 但是，不能把特殊指令去掉然后重新放到p中
				// 因为Read是调用 底层的tls的，tls会把 特殊指令 以及跟着的直连数据一起解密；
				//	我们需要从TeeConn的Recorder中读取真实的数据, 也是在main.go里操作

				return

			}

		}*/
	return
}

// DetectReader 对每个Read的数据进行分析，判断是否是tls流量
type DetectWriter struct {
	io.Writer
	ComDetectStruct
}

// 直接写入，而不进行探测
func (dw *DetectWriter) SimpleWrite(p []byte) (n int, err error) {
	n, err = dw.Writer.Write(p)
	return
}

var SpecialCommand = []byte{1, 2, 3, 4}

//发现，数据基本就是 23 3 3， 22 3 3，22 3 1 ， 20 3 3
//  一个首包不为23 3 3 的包往往会出现在 1184长度的包的后面，而且一般 1184长度的包 的开头是 22 3 3 0 122，且总是在Write里面发生
//  所以可以直接推测这个就是握手包; 实测 22 3 3 0 122 开头的，无一例外都是 1184长度，且后面接多个 开头任意的 Write
// 也有一些特殊情况，比如 22 3 1 首部的包，往往是 517 长度，后面也会紧跟着一些首部不为 22/23 的 Write
//
// 23 3 3 也是有可能 发生后面不为22/23的write，长度 不等
//
// 总之，我们在客户端的 Write 操作，就是 外界试图使用我们的 Write 写入数据
//  所以在socks5后面 使用的这个 Write， 应该是把 服务端的响应 写回 socks5，比如 serverhello 之类
//  	服务端的 Write 操作，也是读 serverhello
//
// 根据之前讨论，23 3 3 就是 数据部分,TLSCiphertext
//  https://halfrost.com/https_record_layer/
// 总之我们依然判断 23 3 3 好了，没那么多技巧，先判断是否存在握手包，握手完成后，遇到23 3 3 后，直接就
//  进入direct模式;
func (dw *DetectWriter) Write(p []byte) (n int, err error) {
	//write和Read不一样，Write是，p现在就具有所有的已知信息，所以我们先过滤，然后再发送到远端

	if dw.IsTls {
		n, err = dw.Writer.Write(p)
		return
	}

	commonFilterStep(nil, &dw.ComDetectStruct, p, false)

	// 经过判断之后，确认是 tls了，则我们缓存这个记录， 然后通过tls发送特殊指令
	if dw.IsTls {

		if dw.isclient {
			// 客户端 DetectConn的Write被调用的话，那就是从 服务端的 tls连接中 提取出了新数据，准备通过socks5发往浏览器
			//
			//客户端判断出IsTLS，那就是收到了特殊指令，p的头部就是特殊指令；p后面可能还跟 数据
			//  然而，一旦后面跟了数据，就完蛋了，这说明特殊指令 和 直连数据连在一起 整个被tls处理 了

			//所以，关键点是，在客户端的tls的前面 也要进行过滤
			// 就是因为 我们需要splice，所以原始数据和 tls解密过的数据 我们都要！

			//客户端发现 浏览器 发送来的数据是 tls数据后， 要使用tls 发送特殊指令，然后，
			// 服务端 在收到 特殊指令之前，已经收到了 tls前面的握手数据，已经处于 handshakePass的阶段
			// 因此，服务端只要在 handshakePass 后面，把 原本的 net.TCPConn 做成一个 TeeReader, 然后给tls真正提供的
			//  实际上是这个TeeReader，这样tls只要Read，则我们的TeeReader也会跟着 输出数据
			//  然后就算tls读到了 直连数据 也没关系，因为我们的TeeReader也读到了！所以根本不必担心丢失数据的问题

			// 总之就是，客户端的wrc的 tls 和 服务端的 wlc 的tls，我们都不直接提供tcp连接，而是包一层

			//  如果特殊指令后面跟着 直连数据一起被tls 读到了的话
			//  那么 原始的数据会包含至少两个  tls record，第一个tls record 解密出来 就是我们的特殊指令，而第二个以后的 record 则是我们要直连的数据

			//	我们需要从TeeConn的Recorder中读取真实的数据

			return

		} else {
			// 服务端Write被调用，那就是 获取到了远程服务器的数据，准备发向 客户端，此时我们发送特殊命令

			// 查看golang源码，可以发现握手过程之后，tls的发送是没有缓存的，只要对 tls.Conn 调用一次Write，就会调用一次或多次net.Conn.Write;
			// 所以我们的 特殊指令 是完全包含在一个完整的 tls record 被发出的。

			// 此处也是不需要保存 要直连发送的数据p 的，在main.go里去直接操作就行

			n, err = dw.Writer.Write(SpecialCommand)

			if err == nil {
				n = len(p)

			}
			return

		}

	}
	n, err = dw.Writer.Write(p)
	//log.Println("DetectWriter,W, 原本", len(p), "实际写入了", n)

	return
}

func GetTlsRecordNextIndex(p []byte) int {
	if len(p) < 5 {
		return -1
	}

	supposedLen := int(p[3])<<8 + int(p[4])

	return 5 + supposedLen

}

func GetLastTlsRecordTailIndex(p []byte) (last_cursor int, count int) {

	//log.Println("总长", len(p))
	cursor := 0
	for {
		if cursor > len(p) {
			return
		}
		index := GetTlsRecordNextIndex(p[cursor:])
		//log.Println("此记录index", index, last_cursor, cursor)
		count++

		if last_cursor > len(p) {
			return
		} else if index < 0 {
			return
		}
		last_cursor = cursor
		cursor += index

	}

}
