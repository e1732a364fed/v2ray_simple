package tlsLayer

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

var PDD bool //print tls detect detail
var OnlyTest bool

func init() {
	log.SetOutput(os.Stdout) //主要是日志太多，如果都能直接用管道放到文件中就好了，默认不是Stdout所以优点尴尬，操作麻烦点

	flag.BoolVar(&PDD, "pdd", false, "print tls detect detail")
	flag.BoolVar(&OnlyTest, "ot", false, "only detect tls, doesn't actually mark tls")

}

// 用于 探测 承载数据是否使用了tls
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

//可选两个参数传入，优先使用rw ，为nil的话 再使用oldConn，作为 底层Read 和Write的 主体
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

	cc.R.isClient = isclient
	cc.W.isClient = isclient

	if netConn := oldConn.(*net.TCPConn); netConn != nil {
		//log.Println("get netConn!")	// 如果是客户端的socks5，网页浏览的话这里一定能转成 TCPConn
		cc.RawConn = netConn
	}

	return cc
}

type ComDetectStruct struct {
	IsTls            bool
	DefinitelyNotTLS bool

	packetCount int
	isClient    bool //client读取要判断 serverhello，server读取要判断 client hello；Write则反过来。

	handShakePass bool

	handshakeFailReason int
}

func (c *ComDetectStruct) incr() {
	c.packetCount++
}

// DetectReader 对每个Read的数据进行分析，判断是否是tls流量
type DetectReader struct {
	io.Reader

	ComDetectStruct
}

// 总之，如果读写都用同样的判断代码的话，客户端和服务端应该就能同步进行 相同的TLS判断
func commonDetect(dr *ComDetectStruct, p []byte, isRead bool) {

	// 首先判断握手包，即第一个包
	//The Client Hello messages contain 01 in the sixth data byte of the TCP packet.
	// 应该是这里定义的： https://datatracker.ietf.org/doc/html/rfc5246#section-7.4
	//

	/*
		//不过，首先要包在 Plaintex结构里
		//https://datatracker.ietf.org/doc/html/rfc5246

		struct {
				ContentType type; //第一字节
				ProtocolVersion version;//第23
				uint16 length;// 第45字节
				opaque fragment[TLSPlaintext.length];
			} TLSPlaintext;

		//ContentType 中，handshake始终是22，也就是说，tls连接的hello字节第一个肯定是22
		// 从tls1.0 到 1.3， 这个情况都是一样的

		tls1.0, tls1.2:  change_cipher_spec(20), alert(21), handshake(22),application_data(23)


	*/

	/*
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

	*/

	n := len(p)

	defer dr.incr()

	p0 := p[0]

	if dr.packetCount == 0 {

		if n < 11 {
			dr.handshakeFailReason = 1
			return
		}
		if p0 != 22 {
			dr.handshakeFailReason = 2
			return
		}

		//version: p9, p10 , 3,1 3,2 3,3 3,4
		if p[9] != 3 {
			dr.handshakeFailReason = 4
			return
		}
		if p[10] == 0 || p[10] > 4 {
			dr.handshakeFailReason = 5
			return
		}

		var shouldCheck_clientHello bool
		var shouldCheck_serverHello bool

		if isRead {
			shouldCheck_clientHello = true
		} else {
			shouldCheck_serverHello = true
		}

		if shouldCheck_clientHello {
			//log.Println("shouldCheck_clientHello")

			if p[5] != 1 { //第六字节，即client_hello
				dr.handshakeFailReason = 3
				return
			}

			dr.handShakePass = true
			return
		} else if shouldCheck_serverHello {

			if p[5] != 2 { //第六字节，即 server_hello
				dr.handshakeFailReason = 3
				return
			}
			dr.handShakePass = true
			return
		}

	} else if !dr.handShakePass {
		dr.DefinitelyNotTLS = true
		return
	}

	p1 := p[1]
	p2 := p[2]

	/*
		if p0 == 22 || p0 == 23 || p0 == 20 || (p0 == 21 && n == 31) {
			//客户端Read 时 少数情况首部会有21，首部为  [21 3 3 0 26 0 0 0 0 0], 一般总长度为31
			// 其它都是 能被捕捉到的。
			if p[1] == 3 {
				dr.IsTls = true
				return
			}
		}*/

	// 23表示 数据层，第二字节3是固定的，然后第3字节 从1到4，表示tls版本，不过tls1.3 和 1.2这里都是 23 3 3
	if p0 == 23 && p1 == 3 && p2 > 0 && p2 < 4 {
		if PDD {
			str := "W"
			if isRead {
				str = "R"
			}
			log.Println(str, "got TLS!")
		}

		if !OnlyTest {
			dr.IsTls = true
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

// 总之，我们在客户端的 Read 操作，就是 我们试图使用 Read 读取客户的请求，然后试图发往 外界
//  所以在socks5后面 使用的这个 Read，是读取客户端发送的请求，比如 clienthello之类
//	服务端的 Read 操作，也是读 clienthello，因为我们总是判断客户传来的数据
func (dr *DetectReader) Read(p []byte) (n int, err error) {
	n, err = dr.Reader.Read(p)
	if !OnlyTest && (dr.DefinitelyNotTLS || dr.IsTls) { //确定了是TLS 或者肯定不是 tls了的话，就直接return掉
		return
	}
	if err != nil {
		eStr := err.Error()
		if strings.Contains(eStr, "use of closed") || strings.Contains(eStr, "reset by peer") || strings.Contains(eStr, "EOF") {
			return
		}
	}
	if !OnlyTest && dr.packetCount > 8 {
		//都8个包了还没断定tls？直接推定不是！
		return
	}

	if n > 3 {
		commonDetect(&dr.ComDetectStruct, p, true)
	}

	return
}

// DetectReader 对每个Read的数据进行分析，判断是否是tls流量
type DetectWriter struct {
	io.Writer
	ComDetectStruct
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
func (dr *DetectWriter) Write(p []byte) (n int, err error) {
	n, err = dr.Writer.Write(p)
	if !OnlyTest && (dr.DefinitelyNotTLS || dr.IsTls) { //确定了是TLS 或者肯定不是 tls了的话，就直接return掉
		return
	}
	if err != nil {
		eStr := err.Error()
		if strings.Contains(eStr, "use of closed") || strings.Contains(eStr, "reset by peer") || strings.Contains(eStr, "EOF") {
			return
		}
	}
	if !OnlyTest && dr.packetCount > 8 {
		//都8个包了还没断定tls？直接推定不是！
		return
	}

	if n > 3 {
		commonDetect(&dr.ComDetectStruct, p, false)
	}

	return
}
