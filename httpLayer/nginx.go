package httpLayer

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (

	//符合 nginx返回的时间格式，且符合 golang对时间格式字符串的 "123456"的约定 的字符串。
	Nginx_timeFormatStr = "02 Jan 2006 15:04:05 MST"

	Nginx400_html = "<html>\r\n<head><title>400 Bad Request</title></head>\r\n<body>\r\n<center><h1>400 Bad Request</h1></center>\r\n<hr><center>nginx/1.21.5</center>\r\n</body>\r\n</html>\r\n"

	// real nginx response,to generate it,  echo xx | nc 127.0.0.1 80 > response
	Err400response_nginx = "HTTP/1.1 400 Bad Request\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/html\r\nConnection: close\r\n\r\n" + Nginx400_html

	// real nginx response,to generate it,  curl -iv --raw 127.0.0.1/not_exist_path > response
	Err404response_nginx = "HTTP/1.1 404 Not Found\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: 19\r\nConnection: keep-alive\r\nCache-Control: no-cache, no-store, no-transform, must-revalidate, private, max-age=0\r\nExpires: Thu, 01 Jan 1970 08:00:00 AWST\r\nPragma: no-cache\r\nVary: Origin\r\nX-Content-Type-Options: nosniff\r\n\r\n404 page not found\n"

	//nginx应该是可以专门配置一个404页面的，所以没有该页面时，就会返回 404 page not found

	Nginx403_html = "<html>\r\n<head><title>403 Forbidden</title></head>\r\n<body bgcolor=\"white\">\r\n<center><h1>403 Forbidden</h1></center>\r\n<hr><center>nginx/1.21.5</center>\r\n</body>\r\n</html>\r\n"

	/* real nginx response, to generate it,  set nginx config like:
	location / {
		return 403;
	}
	*/
	Err403response_nginx = "HTTP/1.1 403 Forbidden\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/html\r\nContent-Length: 169\r\nConnection: keep-alive\r\n\r\n" + Nginx403_html

	//备注
	// 1. vim中， "\r" 显示为 ^M, 输入它是用 ctrl + V + M
	// 2. vim 在显示 末尾 有 \n 的文件 时， 会 直接省略这个 \n
)

var (
	nginxTimezone = time.FixedZone("GMT", 0)

	bs_Nginx403_html = []byte(Nginx403_html)
	bs_Nginx400_html = []byte(Nginx400_html)
)

//Get real a 400 response that looks like it comes from nginx.
func GetNginx400Response() string {
	return GetNginxResponse(Err400response_nginx)
}

//Get real a 403 response that looks like it comes from nginx.
func GetNginx403Response() string {
	return GetNginxResponse(Err403response_nginx)
}

//Get real a 404 response that looks like it comes from nginx.
func GetNginx404Response() string {
	return GetNginxResponse(Err404response_nginx)
}

func GetNginxWeekdayStr(t *time.Time) string {
	return t.Weekday().String()[:3]
}

//Get real a response that looks like it comes from nginx.
func GetNginxResponse(template string) string {
	t := time.Now().UTC().In(nginxTimezone)

	tStr := t.Format(Nginx_timeFormatStr)
	str := strings.Replace(template, Nginx_timeFormatStr, tStr, 1)
	str = strings.Replace(str, "Sat", GetNginxWeekdayStr(&t), 1)

	return str
}

//mimic GetNginx400Response()
func SetNginx400Response(rw http.ResponseWriter) {

	rw.Header().Add("Server", "nginx/1.21.5")
	rw.Header().Add("Content-Type", "text/html")
	rw.Header().Add("Connection", "close")

	t := time.Now().UTC().In(nginxTimezone)
	tStr := t.Format(Nginx_timeFormatStr)
	tStr = GetNginxWeekdayStr(&t) + ", " + tStr

	rw.Header().Add("Date", tStr)

	//rw.Header().Add("Content-Length", strconv.Itoa(len(bs_Nginx400_html)))//真实nginx 400响应里不含 Content-Length
	// 情况是这样的： 如果 返回的是html，则 Connection 是 keep-alive, 并且 有 Content-Length；
	// 如果返回的是纯字符串，则Connection 是 Close，并 没有 Content-Length；

	rw.WriteHeader(http.StatusBadRequest)

	rw.Write(bs_Nginx400_html)
	if flusher, ok := rw.(http.Flusher); ok {
		flusher.Flush()
	}

}

func SetNginx403Response(rw http.ResponseWriter) {
	rw.Header().Add("Server", "nginx/1.21.5")
	rw.Header().Add("Content-Type", "text/html")
	rw.Header().Add("Connection", "keep-alive")

	t := time.Now().UTC().In(nginxTimezone)
	tStr := t.Format(Nginx_timeFormatStr)
	tStr = GetNginxWeekdayStr(&t) + ", " + tStr

	rw.Header().Add("Date", tStr)

	rw.Header().Add("Content-Length", strconv.Itoa(len(bs_Nginx403_html)))

	rw.WriteHeader(http.StatusForbidden)

	rw.Write(bs_Nginx403_html)
	if flusher, ok := rw.(http.Flusher); ok {
		flusher.Flush()
	}

}

type RejectConn struct {
	http.ResponseWriter
}

func (RejectConn) RejectBehaviorDefined() bool {

	return true
}
func (rc RejectConn) Reject() {
	SetNginx403Response(rc.ResponseWriter)

}
