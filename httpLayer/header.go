package httpLayer

import (
	"bytes"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

/*
观察v2ray的实现，在没有header时，还会添加一个 Date ，这个v2ray的文档里没提

v2ray文档: https://www.v2fly.org/config/transport/tcp.html#noneheaderobject

相关 v2ray代码: transport/internet/headers/http/http.go
*/

type RequestHeader struct {
	Version string              `toml:"version"` //默认值为 "1.1"
	Method  string              `toml:"method"`  //默认值为 "GET"。
	Path    []string            `toml:"path"`    //默认值为 ["/"]。当有多个值时，每次请求随机选择一个值。
	Headers map[string][]string `toml:"headers"` //一个键值对，每个键表示一个 HTTP 头的名称，对应的值是一个数组。每次请求会附上所有的键，并随机选择一个对应的值。
}

type ResponseHeader struct {
	Version    string              `toml:"version"` // 1.1
	StatusCode string              `toml:"status"`  // 200
	Reason     string              `toml:"reason"`  // OK
	Headers    map[string][]string `toml:"headers"`
}

//http 头 预设, 分客户端的request 和 服务端的 response这两部分.
type HeaderPreset struct {
	Request  *RequestHeader  `toml:"request"`
	Response *ResponseHeader `toml:"response"`
}

//默认值保持与v2ray的配置相同
func (hh *HeaderPreset) AssignDefaultValue() {
	if hh.Request == nil {
		hh.Request = &RequestHeader{}
	}
	if hh.Request.Version == "" {
		hh.Request.Version = "1.1"
	}
	if hh.Request.Method == "" {
		hh.Request.Method = "GET"
	}
	if len(hh.Request.Path) == 0 {
		hh.Request.Path = []string{"/"}
	}
	if len(hh.Request.Headers) == 0 {
		hh.Request.Headers = map[string][]string{
			"Host":            {"www.baidu.com", "www.bing.com"},
			"User-Agent":      {"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36", "Mozilla/5.0 (iPhone; CPU iPhone OS 10_0_2 like Mac OS X) AppleWebKit/601.1 (KHTML, like Gecko) CriOS/53.0.2785.109 Mobile/14A456 Safari/601.1.46"},
			"Accept-Encoding": {"gzip, deflate"},
			"Connection":      {"keep-alive"},
			"Pragma":          {"no-cache"},
		}

	}

	if hh.Response == nil {
		hh.Response = &ResponseHeader{}
	}

	if hh.Response.Version == "" {
		hh.Response.Version = "1.1"
	}

	if hh.Response.StatusCode == "" {
		hh.Response.StatusCode = "200"
	}
	if hh.Response.Reason == "" {
		hh.Response.Reason = "OK"
	}

	if len(hh.Response.Headers) == 0 {
		hh.Response.Headers = map[string][]string{
			"Content-Type":      {"application/octet-stream", "video/mpeg"},
			"Transfer-Encoding": {"chunked"},
			"Connection":        {"keep-alive"},
			"Pragma":            {"no-cache"},
		}
	}
}

func (h *HeaderPreset) ReadRequest(underlay net.Conn) (err error, leftBuf *bytes.Buffer) {

	var rp H1RequestParser
	err = rp.ReadAndParse(underlay)
	if err != nil {
		return
	}
	if rp.Method != h.Request.Method {
		err = utils.ErrInErr{ErrDesc: "ReadRequest failed, wrong method", ErrDetail: utils.ErrInvalidData, Data: rp.Method}
		return
	}

	if rp.Version != h.Request.Version {
		err = utils.ErrInErr{ErrDesc: "ReadRequest failed, wrong version", ErrDetail: utils.ErrInvalidData, Data: rp.Version}
		return
	}

	if !slices.Contains(h.Request.Path, rp.Path) {
		err = utils.ErrInErr{ErrDesc: "ReadRequest failed, wrong path", ErrDetail: utils.ErrInvalidData, Data: rp.Path}
		return
	}
	allbytes := rp.WholeRequestBuf.Bytes()
	indexOfEnding := bytes.Index(allbytes, HeaderENDING_bytes)
	if indexOfEnding < 0 {
		err = utils.ErrInvalidData
		return

	}
	headerBytes := rp.WholeRequestBuf.Next(indexOfEnding)

	indexOfFirstCRLF := bytes.Index(allbytes, []byte(CRLF))

	headerBytes = headerBytes[indexOfFirstCRLF+2:]

	headerBytesList := bytes.Split(headerBytes, []byte(CRLF))
	matchCount := 0
	for _, header := range headerBytesList {
		//log.Println("ReadRequest read header", string(h))
		hs := string(header)
		ss := strings.Split(hs, ":")
		if len(ss) != 2 {
			err = utils.ErrInvalidData
			return
		}
		key := strings.TrimLeft(ss[0], " ")
		value := strings.TrimLeft(ss[1], " ")

		thisList := h.Request.Headers[key]
		if len(thisList) == 0 {

			switch key {
			case "Content-Length", "Date":
				//go官方包会主动添加 Content-Length
				// 而 v2ray 会 主动添加 Date
			default:

				err = utils.ErrInErr{ErrDesc: "ReadRequest failed, unknown header", ErrDetail: utils.ErrInvalidData, Data: hs}
				return
			}

		} else {
			var ok bool
			for _, v := range thisList {
				if value == strings.TrimLeft(v, " ") {
					ok = true
					matchCount++
					break
				}
			}
			if !ok {
				err = utils.ErrInErr{ErrDesc: "ReadRequest failed, header content not match", ErrDetail: utils.ErrInvalidData, Data: hs}
				return
			}
		}

	} //for headerBytesList
	if diff := len(h.Request.Headers) - matchCount; diff > 0 {
		err = utils.ErrInErr{ErrDesc: "ReadRequest failed, not all headers given", ErrDetail: utils.ErrInvalidData, Data: diff}
		return
	}

	rp.WholeRequestBuf.Next(4)

	return nil, rp.WholeRequestBuf
}

func (p *HeaderPreset) WriteRequest(underlay net.Conn, payload []byte) error {

	buf := bytes.NewBuffer(payload)
	r, err := http.NewRequest(p.Request.Method, p.Request.Path[0], buf)
	if err != nil {
		return err
	}

	nh := maps.Clone(p.Request.Headers)
	r.Header = nh

	for k, v := range nh {
		nh[k] = []string{v[rand.Intn(len(v))]}

	}

	hlist := nh["Host"]
	r.Host = hlist[rand.Intn(len(hlist))]

	return r.Write(underlay)
}

func (p *HeaderPreset) ReadResponse(underlay net.Conn) (err error, leftBuf *bytes.Buffer) {

	bs := utils.GetPacket()
	var n int
	n, err = underlay.Read(bs)
	if err != nil {

		return
	}

	//response我们就直接默认肯定ok了，毕竟只要能收到我们的服务器发来的response，
	// 就证明我们服务器验证我们发送过的request已经通过了。
	//而且header都是我们自己配置的，也没有任何新信息

	indexOfEnding := bytes.Index(bs[:n], HeaderENDING_bytes)
	if indexOfEnding < 0 {
		err = utils.ErrInvalidData
		return

	}

	buf := bytes.NewBuffer(bs[indexOfEnding+4 : n])

	return nil, buf
}

func (p *HeaderPreset) WriteResponse(underlay net.Conn, payload []byte) error {
	buf := utils.GetBuf()

	buf.WriteString("HTTP/")
	buf.WriteString(p.Response.Version)
	buf.WriteString(" ")
	buf.WriteString(p.Response.StatusCode)
	buf.WriteString(" ")
	buf.WriteString(p.Response.Reason)
	buf.WriteString(CRLF)

	for key, v := range p.Response.Headers {
		thisStr := v[rand.Intn(len(v))]
		buf.WriteString(key)
		buf.WriteString(":")
		buf.WriteString(thisStr)
		buf.WriteString(CRLF)

	}
	buf.WriteString(CRLF)

	_, err := buf.WriteTo(underlay)
	utils.PutBuf(buf)

	if err != nil {
		return err
	}

	if len(payload) > 0 {
		_, err = underlay.Write(payload)
	}
	return err
}

type HeaderConn struct {
	net.Conn
	H           *HeaderPreset
	IsServerEnd bool

	optionalReader io.Reader

	notFirstWrite bool
}

func (pc *HeaderConn) Read(p []byte) (n int, err error) {
	var buf *bytes.Buffer

	if pc.IsServerEnd {
		if pc.optionalReader == nil {
			err, buf = pc.H.ReadRequest(pc.Conn)
			if err != nil {
				err = utils.ErrInErr{ErrDesc: "http HeaderConn Read failed, at serverEnd", ErrDetail: err}
				return
			}

			pc.optionalReader = io.MultiReader(buf, pc.Conn)
		}

	} else {
		if pc.optionalReader == nil {
			err, buf = pc.H.ReadResponse(pc.Conn)
			if err != nil {
				err = utils.ErrInErr{ErrDesc: "http HeaderConn Read failed", ErrDetail: err}
				return
			}

			pc.optionalReader = io.MultiReader(buf, pc.Conn)
		}
	}
	return pc.optionalReader.Read(p)

}

func (pc *HeaderConn) Write(p []byte) (n int, err error) {

	if pc.IsServerEnd {
		if pc.notFirstWrite {
			return pc.Conn.Write(p)
		}
		pc.notFirstWrite = true
		err = pc.H.WriteResponse(pc.Conn, p)

	} else {

		if pc.notFirstWrite {
			return pc.Conn.Write(p)
		}
		pc.notFirstWrite = true
		err = pc.H.WriteRequest(pc.Conn, p)
	}

	if err != nil {
		return
	}

	n = len(p)
	return

}
