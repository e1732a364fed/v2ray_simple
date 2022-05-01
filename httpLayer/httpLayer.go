/*
Package httpLayer provides methods and definitions for http layer.

Fallback 由 本包 处理. 因为回落的目标只可能是http服务器.

http头 格式 可以参考：

https://datatracker.ietf.org/doc/html/rfc2616#section-4

https://datatracker.ietf.org/doc/html/rfc2616#section-5
*/
package httpLayer

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/e1732a364fed/v2ray_simple/utils"

	"net/http"
	"net/http/httptest"
)

var Err404response = `HTTP/1.1 404 Not Found\r\nContent-Type: text/html
Connection: keep-alive\r\n404 Not Found\r\n`

const Err403response = `HTTP/1.1 403 Forbidden
Connection: close
Cache-Control: max-age=3600, public
Content-Length: 0

`

const (
	H11_Str = "http/1.1"
	H2_Str  = "h2"

	CRLF = "\r\n"

	//参考 https://datatracker.ietf.org/doc/html/rfc2616#section-4.1
	//
	//http头的尾部. 每个header末尾都有一个 crlf, 整个头部结尾还有一个crlf, 所以是两个.
	HeaderENDING = CRLF + CRLF

	//我们不使用v2ray的8k的最大header长度限制，因为这反倒会探测出特殊性。http标准是最大1MB。
)

var (
	HeaderENDING_bytes = []byte(HeaderENDING)

	ErrNotHTTP_Request = errors.New("not http request")
)

func init() {
	//使用 httptest 包 来完美重现 golang的http包的 notfound, 可以随golang具体实现而同步变化

	req := httptest.NewRequest("GET", "http://exam.com/", nil)
	w := httptest.NewRecorder()
	http.NotFound(w, req)

	buf := &bytes.Buffer{}
	w.Result().Write(buf)

	Err404response = buf.String()

}

type RequestErr struct {
	Path   string
	Method string
}

func (e *RequestErr) Is(err error) bool {
	if err == nil {
		return false
	}
	if re, ok := err.(*RequestErr); ok && re != nil {
		return (e.Path == re.Path && e.Method == re.Path)
	}
	if err == utils.ErrInvalidData {
		return true
	}
	return false
}

func (pe *RequestErr) Error() string {
	var sb strings.Builder
	sb.WriteString("InvaidRequest ")
	sb.WriteString(pe.Method)
	sb.WriteString(",")
	sb.WriteString(pe.Path)

	return sb.String()
}

// H1RequestParser被用于 预读一个链接，判断该连接是否是有效的http请求,
// 并将Version，Path，Method 记录在结构中.
//
// 只能过滤 http 0.9, 1.0 和 1.1. 无法过滤h2和h3.
type H1RequestParser struct {
	Version         string
	Path            string
	Method          string
	WholeRequestBuf *bytes.Buffer
	Failreason      int //为0表示没错误
}

// 尝试读取数据并解析HTTP请求, 解析道道 数据会存入 RequestParser 结构中.
//如果读取错误,会返回该错误; 如果读到的不是HTTP请求，返回 ErrNotHTTP_Request;
func (rhr *H1RequestParser) ReadAndParse(r io.Reader) error {
	bs := utils.GetPacket()

	n, e := r.Read(bs)
	if e != nil {
		return e
	}
	data := bs[:n]
	buf := bytes.NewBuffer(data)
	rhr.WholeRequestBuf = buf

	rhr.Version, rhr.Method, rhr.Path, rhr.Failreason = GetRequestMethod_and_PATH_from_Bytes(data, false)
	if rhr.Failreason != 0 {
		return utils.ErrInErr{ErrDesc: "httpLayer ReadAndParse failed", ErrDetail: ErrNotHTTP_Request, Data: rhr.Failreason}
	}
	return nil
}

// http level fallback metadata
type FallbackMeta struct {
	net.Conn
	H1RequestBuf *bytes.Buffer
	Path         string
	Method       string
	IsH2         bool

	H2Request *http.Request
}
