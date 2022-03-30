//Package http implements an http proxy server
package http

import (
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/hahahrfool/v2ray_simple/httpLayer"
	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
)

const name = "http"

func init() {
	proxy.RegisterServer(name, &ServerCreator{})
}

type ServerCreator struct{}

//只有地址和port需要配置，非常简单
func (_ ServerCreator) NewServerFromURL(u *url.URL) (proxy.Server, error) {

	// TODO: Support Basic Auth

	s := &Server{}
	return s, nil
}

func (_ ServerCreator) NewServer(dc *proxy.ListenConf) (proxy.Server, error) {

	s := &Server{}
	return s, nil
}

type Server struct {
	proxy.ProxyCommonStruct
}

func (_ Server) CanFallback() bool {
	return false //true //暂时不考虑回落，下次再说
}

func (_ Server) Name() string {
	return name
}

func (s *Server) Handshake(underlay net.Conn) (newconn io.ReadWriteCloser, targetAddr *netLayer.Addr, err error) {
	var b = utils.GetMTU() //一般要获取请求信息，不需要那么长; 就算是http，加了path，也不用太长
	//因为要储存为 firstdata，所以也无法直接放回

	newconn = underlay

	n := 0

	n, err = underlay.Read(b[:])
	if err != nil {
		return
	}

	//rfc: https://datatracker.ietf.org/doc/html/rfc7231#section-4.3.6
	// "CONNECT is intended only for use in requests to a proxy.  " 总之CONNECT命令专门用于代理.
	// GET如果 path也是带 http:// 头的话，也是可以的

	method, path, failreason := httpLayer.GetRequestMethod_and_PATH_from_Bytes(b[:n], true)
	if failreason != 0 {
		err = utils.NewDataErr("get method/path failed, method:"+method+" ,reason:", nil, failreason)

		//一个正常的http代理如果遇到了 格式不符的情况的话是要返回 400 等错误代码的
		// 但是，也不能说不返回400的就是异常服务器，因为这可能是服务器自己的策略，无视一切错误请求，比如防黑客时就常常会如此.
		// 所以我们就直接return即可
		//
		//不过另外注意，连method都没有，那么就没有回落的可能性
		return
	}

	//log.Println("GetRequestMethod_and_PATH_from_Bytes", method, URL, "data:", string(b[:n]))

	var isHTTPS bool

	// 如果方法是CONNECT，则为https协议的代理
	if method == "CONNECT" {
		isHTTPS = true
	}

	var addressStr string

	if isHTTPS {
		addressStr = path //实测都会自带:443, 也就不需要我们额外判断了

	} else {

		hostPortURL, err2 := url.Parse(path)
		if err2 != nil {
			err = err2
			return
		}
		addressStr = hostPortURL.Host

		if strings.Index(hostPortURL.Host, ":") == -1 { //host不带端口， 默认80
			addressStr = hostPortURL.Host + ":80"
		}
	}

	targetAddr, err = netLayer.NewAddr(addressStr)
	if err != nil {
		return
	}
	//如果使用https协议，需先向客户端表示连接建立完毕
	if isHTTPS {
		underlay.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		//这个也是https代理的特征，所以不适合 公网使用
		//正常来说我们的服务器要先dial，dial成功之后再返回200，但是因为我们目前的架构是在main函数里dail，
		// 所以就直接写入了.

		//另外，nginx是没有实现 CONNECT的，不过有插件

	} else {
		newconn = &ProxyConn{
			firstData: b[:n],
			Conn:      underlay,
		}

	}
	return
}

//用于纯http的 代理，dial后，第一次要把客户端的数据原封不动发送给远程服务端
// 就是说，第一次从 ProxyConn Read时，读到的一定是之前读过的数据，原理有点像 fallback
type ProxyConn struct {
	net.Conn
	firstData []byte
	notFirst  bool
}

func (pc *ProxyConn) Read(p []byte) (int, error) {
	if pc.notFirst {
		return pc.Conn.Read(p)
	}
	bs := pc.firstData
	pc.firstData = nil
	pc.notFirst = true

	n := copy(p, bs)
	utils.PutBytes(bs)
	return n, nil
}

// ReadFrom implements the io.ReaderFrom ReadFrom method.
// 专门用于适配 tcp的splice.
func (pc *ProxyConn) ReadFrom(r io.Reader) (n int64, e error) {

	//pc.Conn肯定不是udp，但有可能是 unix domain socket。暂时先不考虑这种情况

	return pc.Conn.(*net.TCPConn).ReadFrom(r)
}
