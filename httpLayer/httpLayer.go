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
)

const (
	H11_Str = "http/1.1"
	H2_Str  = "h2"
)

var ErrNotHTTP_Request = errors.New("not http request")

type RequestErr struct {
	Path   string
	Method string
}

func (pe *RequestErr) Error() string {
	return "InvaidRequest " + pe.Method + "," + pe.Path
}

type RequestParser struct {
	Path            string
	Method          string
	WholeRequestBuf *bytes.Buffer
	Failreason      int
}

// 尝试读取数据并解析HTTP请求, 解析道道 数据会存入 RequestParser 结构中.
//如果读取错误,会返回错误; 如果读到的不是HTTP请求，返回 ErrNotHTTP_Request;
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
