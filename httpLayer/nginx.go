package httpLayer

import (
	"strings"
	"time"
)

const (

	//符合 nginx返回的时间格式，且符合 golang对时间格式字符串的 "123456"的约定 的字符串。
	nginx_timeFormatStr = "02 Jan 2006 15:04:05 MST"

	// real nginx response, echo xx | nc 127.0.0.1 80 > response
	Err400response_nginx = "HTTP/1.1 400 Bad Request\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/html\r\nConnection: close\r\n\r\n<html>\r\n<head><title>400 Bad Request</title></head>\r\n<body>\r\n<center><h1>400 Bad Request</h1></center>\r\n<hr><center>nginx/1.21.5</center>\r\n</body>\r\n</html>\r\n"

	// real nginx response, curl -iv --raw 127.0.0.1/not_exist_path > response ;
	Err404response_nginx = "HTTP/1.1 404 Not Found\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: 19\r\nConnection: keep-alive\r\nCache-Control: no-cache, no-store, no-transform, must-revalidate, private, max-age=0\r\nExpires: Thu, 01 Jan 1970 08:00:00 AWST\r\nPragma: no-cache\r\nVary: Origin\r\nX-Content-Type-Options: nosniff\r\n\r\n404 page not found\n"

	/* real nginx response, set nginx config like:
	location / {
		return 403;
	}

	*/
	Err403response_nginx = "HTTP/1.1 403 Forbidden\r\nServer: nginx/1.21.5\r\nDate: Sat, 02 Jan 2006 15:04:05 MST\r\nContent-Type: text/html\r\nContent-Length: 183\r\nConnection: keep-alive\r\n\r\n<html>\r\n<head><title>403 Forbidden</title></head>\r\n<body bgcolor=\"white\">\r\n<center><h1>403 Forbidden</h1></center>\r\n<hr><center>nginx/1.21.5</center>\r\n</body>\r\n</html>\r\n"

	//备注
	// 1. vim中， "\r" 显示为 ^M, 输入它是用 ctrl + V + M
	// 2. vim 在显示 末尾 有 \n 的文件 时， 会 直接省略这个 \n
)

var nginxTimezone = time.FixedZone("GMT", 0)

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

//Get real a response that looks like it comes from nginx.
func GetNginxResponse(template string) string {
	t := time.Now().UTC().In(nginxTimezone)

	tStr := t.Format(nginx_timeFormatStr)
	str := strings.Replace(template, nginx_timeFormatStr, tStr, 1)
	str = strings.Replace(str, "Sat", t.Weekday().String()[:3], 1)

	return str
}
