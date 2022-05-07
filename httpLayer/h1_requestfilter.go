package httpLayer

import (
	"bytes"
)

//也许有 ws 的 earlydata放在 query请求里的情况; 虽然本作不支持这种earlydata, 但是也要认定这是合法的请求。
const MaxParseUrlLen = 3000

type RawHeader struct {
	Head  []byte
	Value []byte
}

// 从数据中试图获取 http1.1, http1.0或 http0.9 请求的 version, path, method 和 headers.
// failreason!=0 表示获取失败, 即表示不是合法的h1请求.
//
// 如果是http代理的话，判断方式会有变化,所以需要 isproxy 参数。
func ParseH1Request(bs []byte, isproxy bool) (version, method, path string, headers []RawHeader, failreason int) {

	if len(bs) < 16 { //http0.9 最小长度为16， http1.0及1.1最小长度为18
		failreason = 1
		return
	}

	if bs[4] == '*' {
		failreason = 2 //this method doesn't support h2c
		return

	}
	//http 方法有：GET, POST, HEAD, PUT, DELETE, OPTIONS, CONNECT, PRI

	//看 v2ray的实现，似乎用了循环的方法，从 5 -> 9字节读取，实际上就是考虑到 http方法可以是 3-7字节
	// v2ray的循环是可能有问题的，还是先过滤掉非http方法比较好，否则回落的话，多了一个转换path成字符串的开销
	//我们使用另一种方法，不使用循环 过滤 http方法, 这样同时可以获取到method

	//3字节， Get，Put，（pri被过滤了）
	// 4字节，Post,Head,
	// 5字节
	// 6字节， delete，
	// 7字节，options，connect

	if bs[5] == ' ' {
		// 没有五字节长度的 http请求方法

		//但是 Get / HTTP/1.1 也会遇到第6字节为空格的情况, put/pri 也一样, 所以还要过滤.

		if bs[3] != ' ' {
			failreason = 3
			return
		}
	}

	shouldSpaceIndex := 0

	switch bs[0] {
	case 'G':
		if bs[1] == 'E' && bs[2] == 'T' {
			shouldSpaceIndex = 3
			method = "GET"
		}
	case 'P':
		if bs[1] == 'U' && bs[2] == 'T' {
			shouldSpaceIndex = 3
			method = "PUT"
		} else if bs[1] == 'O' && bs[2] == 'S' && bs[3] == 'T' {
			shouldSpaceIndex = 4
			method = "POST"
		}
	case 'H':
		if bs[1] == 'E' && bs[2] == 'A' && bs[3] == 'D' {
			shouldSpaceIndex = 4
			method = "HEAD"
		}
	case 'D':
		if bs[1] == 'E' && bs[2] == 'L' && bs[3] == 'E' && bs[4] == 'T' && bs[5] == 'E' {
			shouldSpaceIndex = 6
			method = "DELETE"
		}
	case 'O':
		if bs[1] == 'P' && bs[2] == 'T' && bs[3] == 'I' && bs[4] == 'O' && bs[5] == 'N' && bs[6] == 'S' {
			shouldSpaceIndex = 7
			method = "OPTIONS"
		}
	case 'C':
		if bs[1] == 'O' && bs[2] == 'N' && bs[3] == 'N' && bs[4] == 'E' && bs[5] == 'C' && bs[6] == 'T' {
			shouldSpaceIndex = 7
			method = "CONNECT"
			if !isproxy {
				//connect 只可能出现于 proxy代理中
				failreason = 4
			}
		}
	}

	if shouldSpaceIndex == 0 || bs[shouldSpaceIndex] != ' ' {
		failreason = 5
		return
	}
	shouldSlashIndex := shouldSpaceIndex + 1

	if isproxy {
		if method == "CONNECT" {
			//https

		} else {
			//http
			if string(bs[shouldSlashIndex:shouldSlashIndex+7]) != "http://" {
				failreason = 6
				return
			}
		}
	} else {
		if bs[shouldSlashIndex] != '/' {
			failreason = 7
			return
		}
	}

	//一般请求样式类似 GET /sdfdsffs.html HTTP/1.1
	//所以找到第二个空格的位置即可，

	last := len(bs)
	if !isproxy { //如果是代理，则我们要判断整个请求，不能漏掉任何部分
		if last > MaxParseUrlLen {
			last = MaxParseUrlLen
		}
	}

	for i := shouldSlashIndex; i < last; i++ {
		b := bs[i]
		if b == '\r' || b == '\n' {
			failreason = 8
			return
		}
		if b == ' ' {
			// 空格后面至少还有 HTTP/1.1\r\n 这种字样，也就是说空格后长度至少为 10
			//https://stackoverflow.com/questions/25047905/http-request-minimum-size-in-bytes/25065089
			if len(bs)-i-1 < 10 {
				failreason = 9
				return
			}

			path = string(bs[shouldSlashIndex:i])

			if string(bs[i+1:i+5]) != "HTTP" {
				failreason = 10
				return
			}

			version = string(bs[i+6 : i+9])
			if bs[i+9] != '\r' || bs[i+10] != '\n' {
				failreason = -11
				return
			}

			leftBs := bs[i+11:]

			indexOfEnding := bytes.Index(leftBs, HeaderENDING_bytes)
			if indexOfEnding < 0 {
				failreason = -12
				return

			}
			headerBytes := leftBs[:indexOfEnding]
			headerBytesList := bytes.Split(headerBytes, []byte(CRLF))
			for _, header := range headerBytesList {

				ss := bytes.SplitN(header, []byte(":"), 2)
				if len(ss) != 2 {
					failreason = -13
					return
				}
				headers = append(headers, RawHeader{
					Head:  bytes.TrimLeft(ss[0], " "),
					Value: bytes.TrimLeft(ss[1], " "),
				})

			}
			//http1.1 要有 Host 这个header，参考
			// https://stackoverflow.com/questions/25047905/http-request-minimum-size-in-bytes/25065089
			// https://stackoverflow.com/questions/9233316/what-is-the-smallest-possible-http-and-https-data-request

			return
		}
	}
	failreason = last //!isproxy时, 我们只判断了前64字节，如果访问url更长的话，这里还是会返回failreason的
	return

}
