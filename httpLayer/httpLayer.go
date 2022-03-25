/*
Package httpLayer 提供http层的一些方法和定义.

比如fallback一般由httpLayer处理
*/
package httpLayer

import (
	"bytes"
	"errors"
	"io"

	"github.com/hahahrfool/v2ray_simple/utils"

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
)

var ErrNotHTTP_Request = errors.New("not http request")

func init() {
	//使用 httptest 包 来完美重现 golang的http包的 notfound, 可以随golang具体实现而变化

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

func (pe *RequestErr) Error() string {
	return "InvaidRequest " + pe.Method + "," + pe.Path
}

// RequestParser被用于 预读一个链接，判断该连接是否是有效的http请求,
// 并将相关数据记录在结构中.
type RequestParser struct {
	Path            string
	Method          string
	WholeRequestBuf *bytes.Buffer
	Failreason      int
}

// 尝试读取数据并解析HTTP请求, 解析道道 数据会存入 RequestParser 结构中.
//如果读取错误,会返回该错误; 如果读到的不是HTTP请求，返回 ErrNotHTTP_Request;
func (rhr *RequestParser) ReadAndParse(r io.Reader) error {
	bs := utils.GetPacket()

	n, e := r.Read(bs)
	if e != nil {
		return e
	}
	data := bs[:n]
	buf := bytes.NewBuffer(data)
	rhr.WholeRequestBuf = buf

	rhr.Method, rhr.Path, rhr.Failreason = GetRequestMethod_and_PATH_from_Bytes(data, false)
	if rhr.Failreason != 0 {
		return ErrNotHTTP_Request
	}
	return nil
}
