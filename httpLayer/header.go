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

//return a clone of m with headers trimmed to one value
func TrimHeaders(m map[string][]string) (result map[string][]string) {

	result = maps.Clone(m)

	for k, v := range result {
		result[k] = []string{v[rand.Intn(len(v))]}
	}
	return
}

// Algorithm below is like standard textproto/CanonicalMIMEHeaderKey, except
// that it operates with slice of bytes and modifies it inplace without copying. copied from gobwas/ws
func CanonicalizeHeaderKey(k []byte) {

	const (
		toLower = 'a' - 'A'      // for use with OR.
		toUpper = ^byte(toLower) // for use with AND.
	)

	upper := true
	for i, c := range k {
		if upper && 'a' <= c && c <= 'z' {
			k[i] &= toUpper
		} else if !upper && 'A' <= c && c <= 'Z' {
			k[i] |= toLower
		}
		upper = c == '-'
	}
}

//all values in template is given by real
func AllHeadersIn(template map[string][]string, realh http.Header) (ok bool, firstNotMatchKey string) {
	for k, vs := range template {
		containThis := false

		thisReal := realh.Get(k)
		if thisReal == "" {
			firstNotMatchKey = k
			return
		}

		for _, v := range vs {
			if thisReal == v {
				containThis = true
				break
			}
		}

		if !containThis {
			firstNotMatchKey = k
			return
		}
	}
	ok = true
	return
}

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

	Strict                              bool `toml:"strict"`
	NoResponseHeaderWhenGotHttpResponse bool `toml:"no_resp_h_c"` //用于读取 回落到 真实http服务器的情况，此时我们就不用自定义的响应，而是用 真实服务器的响应。no_resp_h_c 意思是 no response header conditional
}

// 将Header改为首字母大写
func (h *HeaderPreset) Prepare() {
	if h.Request != nil && len(h.Request.Headers) > 0 {

		var realHeaders http.Header = make(http.Header)
		for k, vs := range h.Request.Headers {
			for _, v := range vs {
				realHeaders.Add(k, v)

			}
		}

		h.Request.Headers = realHeaders
	}
	if h.Response != nil && len(h.Response.Headers) > 0 {

		var realHeaders http.Header = make(http.Header)
		for k, vs := range h.Response.Headers {
			for _, v := range vs {
				realHeaders.Add(k, v)

			}
		}

		h.Response.Headers = realHeaders
	}
}

//默认值保持与v2ray的配置相同
func (h *HeaderPreset) AssignDefaultValue() {
	if h.Request == nil {
		h.Request = &RequestHeader{}
	}
	if h.Request.Version == "" {
		h.Request.Version = "1.1"
	}
	if h.Request.Method == "" {
		h.Request.Method = "GET"
	}
	if len(h.Request.Path) == 0 {
		h.Request.Path = []string{"/"}
	}
	if len(h.Request.Headers) == 0 {
		h.Request.Headers = map[string][]string{
			"Host":            {"www.baidu.com", "www.bing.com"},
			"User-Agent":      {"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36", "Mozilla/5.0 (iPhone; CPU iPhone OS 10_0_2 like Mac OS X) AppleWebKit/601.1 (KHTML, like Gecko) CriOS/53.0.2785.109 Mobile/14A456 Safari/601.1.46"},
			"Accept-Encoding": {"gzip, deflate"},
			"Connection":      {"keep-alive"},
			"Pragma":          {"no-cache"},
		}

	}

	if h.Response == nil {
		h.Response = &ResponseHeader{}
	}

	if h.Response.Version == "" {
		h.Response.Version = "1.1"
	}

	if h.Response.StatusCode == "" {
		h.Response.StatusCode = "200"
	}
	if h.Response.Reason == "" {
		h.Response.Reason = "OK"
	}

	if len(h.Response.Headers) == 0 {
		h.Response.Headers = map[string][]string{
			"Content-Type":      {"application/octet-stream", "video/mpeg"},
			"Transfer-Encoding": {"chunked"},
			"Connection":        {"keep-alive"},
			"Pragma":            {"no-cache"},
		}
	}

	h.Prepare()
}

func (h *HeaderPreset) ReadRequest(underlay io.Reader) (rp H1RequestParser, leftBuf *bytes.Buffer, err error) {

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

	leftBuf = bytes.NewBuffer(allbytes)

	indexOfEnding := bytes.Index(allbytes, HeaderENDING_bytes)
	if indexOfEnding < 0 {
		err = utils.ErrInvalidData
		return

	}
	headerBytes := leftBuf.Next(indexOfEnding)

	if h.Strict {
		indexOfFirstCRLF := bytes.Index(allbytes, []byte(CRLF))

		headerBytes = headerBytes[indexOfFirstCRLF+2:]

		headerBytesList := bytes.Split(headerBytes, []byte(CRLF))
		matchCount := 0
		for _, header := range headerBytesList {
			//log.Println("ReadRequest read header", string(h))
			hs := string(header)
			ss := strings.SplitN(hs, ":", 2)
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
					// 而 v2ray 会 主动添加 Date, 还会加 User-Agent: Go-http-client/1.1

					//所以最好在配置文件中明示出 自己定义的 User-Agent
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

	}

	leftBuf.Next(len(HeaderENDING))

	return
}

func (h *HeaderPreset) WriteRequest(underlay io.Writer, payload []byte) error {

	buf := bytes.NewBuffer(payload)
	r, err := http.NewRequest(h.Request.Method, h.Request.Path[0], buf)
	if err != nil {
		return err
	}

	nh := TrimHeaders(h.Request.Headers)

	r.Header = nh

	hlist := nh["Host"]
	r.Host = hlist[rand.Intn(len(hlist))]

	return r.Write(underlay)
}

func (h *HeaderPreset) ReadResponse(underlay io.Reader) (leftBuf *bytes.Buffer, err error) {

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

	return buf, nil
}

func (h *HeaderPreset) WriteResponse(underlay io.Writer, payload []byte) error {
	buf := utils.GetBuf()

	buf.WriteString("HTTP/")
	buf.WriteString(h.Response.Version)
	buf.WriteString(" ")
	buf.WriteString(h.Response.StatusCode)
	buf.WriteString(" ")
	buf.WriteString(h.Response.Reason)
	buf.WriteString(CRLF)

	for key, v := range h.Response.Headers {
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
	ReadOkCallback func(*bytes.Buffer)

	notFirstWrite bool
}

func (c *HeaderConn) Read(p []byte) (n int, err error) {
	var buf *bytes.Buffer

	if c.IsServerEnd {
		if c.optionalReader == nil {
			var rp H1RequestParser

			rp, buf, err = c.H.ReadRequest(c.Conn)
			if err != nil {

				const errDesc = "http HeaderConn Read failed, at serverEnd"

				if rp.WholeRequestBuf != nil {

					err = &utils.ErrBuffer{
						Err: utils.ErrInErr{ErrDesc: errDesc, ErrDetail: err},
						Buf: rp.WholeRequestBuf,
					}
				} else {
					err = &utils.ErrInErr{ErrDesc: errDesc, ErrDetail: err}
				}

				return
			}
			if c.ReadOkCallback != nil {
				c.ReadOkCallback(rp.WholeRequestBuf)

			}

			c.optionalReader = io.MultiReader(buf, c.Conn)
		}

	} else {
		if c.optionalReader == nil {
			buf, err = c.H.ReadResponse(c.Conn)
			if err != nil {
				err = utils.ErrInErr{ErrDesc: "http HeaderConn Read failed", ErrDetail: err}
				return
			}

			c.optionalReader = io.MultiReader(buf, c.Conn)
		}
	}
	return c.optionalReader.Read(p)

}

func (c *HeaderConn) Write(p []byte) (n int, err error) {

	if c.IsServerEnd {
		if c.notFirstWrite {
			return c.Conn.Write(p)
		} else {
			c.notFirstWrite = true

			var shouldWriteDirectly bool

			if c.H.NoResponseHeaderWhenGotHttpResponse {
				if len(p) > 5 && string(p[:4]) == "HTTP" {
					shouldWriteDirectly = true
				}
			}

			if shouldWriteDirectly {
				return c.Conn.Write(p)

			} else {
				err = c.H.WriteResponse(c.Conn, p)

			}

		}

	} else {

		if c.notFirstWrite {
			return c.Conn.Write(p)
		}
		c.notFirstWrite = true
		err = c.H.WriteRequest(c.Conn, p)
	}

	if err != nil {
		return
	}

	n = len(p)
	return

}
