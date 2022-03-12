package tlsLayer

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/hahahrfool/v2ray_simple/common"
)

var PDD bool //print tls detect detail
var OnlyTest bool

func init() {
	log.SetOutput(os.Stdout) //主要是日志太多，如果都能直接用管道放到文件中就好了，默认不是Stdout所以优点尴尬，操作麻烦点

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

	supposedLen int //for write
	wholeMsgLen int //for write

	MiddleTlsRecord []byte

	LeftTlsRecord []byte //for write

	needMoreReadLen int //for write

	CutType byte //3种情况

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
			if !isRead {
				//避免客户端只读到了一半 就开始裸奔，要读全，所以注明一下没读到的长度
				supposedLen := int(p[3])<<8 + int(p[4])

				// 不过实测，supposedLen比 实际读取到的还短啊，读到 supposedLen=1203，但是 实际n=9760

				// 实测，supposedLen可能比读到的数据长，也可能比读到的数据短，这都是不一定的，这是tcp自己的性质，没办法

				log.Println("W, supposedLen", supposedLen, n)
				cd.supposedLen = supposedLen

			}
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

	if dw.IsTls && dw.needMoreReadLen != 0 {
		// 上一次读的时候，读了一半的tls包，还需要再读另一半
		//这里默认认为p 给的足够大，能直接就读满这另一半

		//但是这还不够，因为可能又剩下了一串tls数据需要写入，还是要缓存

		diff := len(p) - dw.needMoreReadLen

		if diff >= 0 {

			hasPartLen := len(dw.LeftTlsRecord)

			dw.LeftTlsRecord = dw.LeftTlsRecord[:hasPartLen+len(p)]

			copy(dw.LeftTlsRecord[hasPartLen:], p)

			dw.needMoreReadLen = 0

			//现在我们要有的都有了, 这个曾经被分割的 LeftTlsRecord 现在完整了(不仅完整，而且又多了一段剩余部分)，我们可以和剩余部分一起写入直连了
			// 总之我们必须按照tls 的 record边界 进行 直连

			// 之前的问题就是，判断是tls的瞬间后，还是把数据加密传输了一次，而且还没有尊重tls的record边界
			// 然后下一次传输是 直连的，导致那个 被分割开的 tls的Record 的前一半被加密，后一半被直连，就错误了
			// 这里把第一次的 完整的tls的record部分加密发送，然后这个单独被分割开的 tls record单独处理，记录下来后，直连发送。

			return len(p), nil //此时应该搞定了，
		} else {
			//p 不够长的情况下就再说吧！

			log.Println("DetectWriter,W, IsTLS, 想读另一半tls 但是p不够长")
			return len(p), nil
		}
	}

	commonFilterStep(nil, &dw.ComDetectStruct, p, false)

	if dw.IsTls && !dw.isclient { //客户端不用再检查tls 记录完整性，因为服务端给做好了

		supposeWholeLen := dw.supposedLen + 5

		if supposeWholeLen < len(p) {

			dw.wholeMsgLen = len(p)

			log.Println("DetectWriter,W, IsTLS", "应该具有长度：", dw.supposedLen, "实际剩余长度", len(p)-5, "多了", len(p)-supposeWholeLen)

			//实际上 supposedLen 应该 = 实际长度-5，因为前面五字节是 tls的 record 头部

			cursor := supposeWholeLen
			//var lastcursor int

			zhanbao_count := 0

			//dw.UnwrittenBs = p[supposeWholeLen:]

			var leftSupposedLen int

			fit := false

			for {
				zhanbao_count++

				left := p[cursor:]
				leftLen := len(left)

				if leftLen > 4 {
					if left[0] != 23 {
						log.Println("DetectWriter,W, 首部不为23", zhanbao_count, left[0])
						break
					}

					leftSupposedLen = int(left[3])<<8 + int(left[4])

					diff := leftLen - 5 - leftSupposedLen

					log.Println("DetectWriter,W, 得到粘包", zhanbao_count, "应该具有长度：", leftSupposedLen, "实际长度", leftLen-5, ",多了", diff)

					nextIndex := cursor + 5 + leftSupposedLen

					if diff > 0 { //意味着还有另一个包粘包！

						//lastcursor = cursor
						cursor = nextIndex
						continue
					} else if diff < 0 {

						log.Println("DetectWriter,W, diff", diff, "<0, 最后一个包长度不够")
						//cursor = nextIndex
						break
					} else {
						// 也有正好的情况发生
						fit = true
						break
					}
				} else {
					break
				}

			}
			//循环结束时，情况是：不是完全没有粘包存在，就是最末尾粘了一半的包
			// 实测，只有 完全没有粘包存在时，传输才会完全成功，否则的话，就属于一半加密一半裸奔，浏览器肯定看不懂

			if fit {
				//就算是fit，也不行！！还是因为，为了让客户端的tls能够仅仅读取到一个tls头，必须进行分割
				// 如果直接发送的话，那么客户端永远不会知道到底在第几个 tls record后面开始 后 是 直连数据；
				// 因为客户端收到的record本身就可以被 tcp连接 分割开，因为分割就要继续读，但是却不知道读到第几个；
				// 所以只能进行分割，然后交给上级。
				// 这里直接按 BigCut进行分割，因为这样第二部分就会被通过直连发送，可以少加密一次

				log.Println("DetectWriter,W, fit, 但依然要按照直连分割")

				first := p[:supposeWholeLen]
				leftRecord := p[supposeWholeLen:]
				cp := common.GetPacket() //这个packet长度64k，足够大了
				cp = cp[:len(leftRecord)]
				copy(cp, leftRecord)
				dw.LeftTlsRecord = cp
				dw.CutType = CutType_fit

				n, err = dw.Writer.Write(first)
				if err == nil {
					n = len(p)
				}
				return
			}

			log.Println("DetectWriter,W, supposedLen和整个数据包长度大小不一致,", supposeWholeLen, len(p), ",剩余粘了", zhanbao_count, "个包,最末尾还粘了不完整的数据长度", len(p)-cursor, "而实际最后一个包的长度应该是", leftSupposedLen)

			dw.needMoreReadLen = leftSupposedLen - len(p) - cursor - 5

			first := p[:supposeWholeLen]
			leftRecord := p[supposeWholeLen:]

			n, err = dw.Writer.Write(first) //把完整的一截tls数据 通过tls加密 发送发过去

			//问题出在客户端的读！就算我们按照完整的tls record发送，客户端还是会读到被割裂的包
			// 关键就是我们的客户端不知道边界在哪里。我们只有一个能与客户端配合的，那就是首包；
			// 所以我们 只能 通过tls加密发送第一个tls record，然后其它的记录则统统缓存起来，留着下一次直接直连发送

			log.Println("DetectWriter,W, 先写入完整的 tls record，", supposeWholeLen)

			cp := common.GetPacket() //这个packet长度64k，足够大了
			cp = cp[:len(leftRecord)]
			copy(cp, leftRecord)

			dw.LeftTlsRecord = cp
			dw.CutType = CutType_big

			if err == nil {
				n = len(p) //假装告诉上级调用者，我们写了整个数据；其实只写了一半， 剩下一半要等待下一次Write提供，然后会一起写入 直连
			}

			return
		} else if supposeWholeLen > len(p) {

			diff := supposeWholeLen - len(p)

			log.Println("DetectWriter,W, IsTLS", "应该具有长度：", dw.supposedLen, "实际剩余长度", len(p)-5, "少了", diff)

			// 因为tcp问题，导致只收到了一半的 tls数据，尴尬，需要接着读，也就是说，需要被动等待第二个Write 给我们提供更多数据；
			// 如果不等待的话，那么我们就是只加密了一半的tls数据，然后另一半被直连了，肯定会得到错误结果

			//这个和 上面粘包的情况时一样的，因为粘包到最后还是会分出一个 收到一半包的情况

			dw.needMoreReadLen = diff

			cp := common.GetPacket() //这个packet长度64k，足够大了
			cp = cp[:len(p)]
			copy(cp, p)
			dw.LeftTlsRecord = cp
			dw.CutType = CutType_small

			n = len(p)

			return
		}
	}

	n, err = dw.Writer.Write(p)
	//log.Println("DetectWriter,W, 原本", len(p), "实际写入了", n)

	return
}
