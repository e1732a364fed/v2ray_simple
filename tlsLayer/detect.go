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

	onlyTest bool //如果此开关打开，则不会去真实修改IsTLS，而只是去过滤所有的包。
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
func NewDetectConn(oldConn net.Conn, rw io.ReadWriter, onlyTest bool) *DetectConn {

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

	cc.W.onlyTest = onlyTest
	cc.R.onlyTest = onlyTest

	if netConn := oldConn.(*net.TCPConn); netConn != nil {
		//log.Println("get netConn!")	// 如果是客户端的socks5，网页浏览的话这里一定能转成 TCPConn
		cc.RawConn = netConn
	}

	return cc
}

type ComDetectStruct struct {
	IsTls    bool
	onlyTest bool

	packetCount int
}

// DetectReader 对每个Read的数据进行分析，判断是否是tls流量
type DetectReader struct {
	io.Reader

	ComDetectStruct
}

func commonDetect(dr *ComDetectStruct, p []byte, isRead bool) {

	dr.packetCount++
	p0 := p[0]
	p1 := p[1]
	p2 := p[2]

	/*
		if p0 == 22 || p0 == 23 || p0 == 20 || (p0 == 21 && n == 31) {
			//客户端Read 时 少数情况首部会有21，首部为  [21 3 3 0 26 0 0 0 0 0], 一般总长度为31
			// 其它都是 能被捕捉到的。
			if p[1] == 3 {
				min := 5
				if n < 5 {
					min = n
				}
				log.Println(" TLS R,", n, err, p[:min])
				dr.IsTls = true
				return
			}
		}*/

	if p0 == 23 && p1 == 3 && p2 == 3 {
		if PDD {
			str := "W"
			if isRead {
				str = "R"
			}
			log.Println(str, "got TLS!")
		}

		if !dr.onlyTest {
			dr.IsTls = true
		}

		return
	}
	n := len(p)

	// 打印没过滤到的数据
	if PDD || dr.onlyTest {
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
//	服务端的 Read 操作，是把 远程目标服务器 发来的 数据 发送到 客户端，比如 serverhello 之类
//
//	我们直接判断23 3 3字节，然后直接推定tls！不管三七二十一，判断错误就错误吧！快就得了！
func (dr *DetectReader) Read(p []byte) (n int, err error) {
	n, err = dr.Reader.Read(p)
	if !dr.onlyTest && dr.IsTls {
		return
	}
	if err != nil {
		eStr := err.Error()
		if strings.Contains(eStr, "use of closed") || strings.Contains(eStr, "reset by peer") || strings.Contains(eStr, "EOF") {
			return
		}
	}
	if !dr.onlyTest && dr.packetCount > 8 {
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

//我发现，数据基本就是 23 3 3， 22 3 3，22 3 1 ， 20 3 3
//  一个首包不为23 3 3 的包往往会出现在 1184长度的包的后面，而且一般 1184长度的包 的开头是 22 3 3 0 122，且总是在Write里面发生
//  所以可以直接推测这个就是握手包; 实测 22 3 3 0 122 开头的，无一例外都是 1184长度，且后面接多个 开头任意的 Write
// 也有一些特殊情况，比如 22 3 1 首部的包，往往是 517 长度，后面也会紧跟着一些首部不为 22/23 的 Write
//
// 23 3 3 也是有可能 发生后面不为22/23的write，长度 不等
//
// 总之，我们在客户端的 Write 操作，就是 外界试图使用我们的 Write 写入数据
//  所以在socks5后面 使用的这个Write，应该是把 服务端的响应 写回 socks5，比如 serverhello之类
//  服务端的 Write操作，是把客户端发来的 数据 发送到远程目标服务器，比如 clienthello之类
//
// 根据之前讨论，23 3 3 就是 数据部分,TLSCiphertext
//  https://halfrost.com/https_record_layer/
// 总之我们依然判断 23 3 3 好了，但是不循环判断了，没那么多技巧，先判断是否存在握手包，握手完成后，遇到23 3 3 后，直接就
//  进入direct模式; 目前从简，连握手包都不检测！测错就测错！
func (dr *DetectWriter) Write(p []byte) (n int, err error) {
	n, err = dr.Writer.Write(p)
	if !dr.onlyTest && dr.IsTls {
		return
	}
	if err != nil {
		eStr := err.Error()
		if strings.Contains(eStr, "use of closed") || strings.Contains(eStr, "reset by peer") || strings.Contains(eStr, "EOF") {
			return
		}
	}
	if !dr.onlyTest && dr.packetCount > 8 {
		//都8个包了还没断定tls？直接推定不是！
		return
	}

	if n > 3 {
		commonDetect(&dr.ComDetectStruct, p, false)
	}

	return
}
