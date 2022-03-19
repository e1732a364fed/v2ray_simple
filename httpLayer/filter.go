package httpLayer

// GetRequestMethod_and_PATH_from_Bytes 从一个字节串中试图获取 http请求的 path,和 method.
// failreason!=0 表示获取失败.
// 同时可以用这个方法判断明文 是不是 http1.1, http1.0, http0.9的 http请求
// 如果是http代理的话，判断方式会有变化,所以需要 isproxy 参数
// 此方法亦可以用于 判断一个http请求头部是否合法
func GetRequestMethod_and_PATH_from_Bytes(bs []byte, isproxy bool) (method string, path string, failreason int) {

	if len(bs) < 16 { //http0.9 最小长度为16， http1.0及1.1最小长度为18
		failreason = 1
		return
	}

	if bs[4] == '*' {
		failreason = 2
		return //not h2c

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
		// 但是也会匹配到4字节+根目录的情况：
		// 如果是 POST / HTTP/1.1, 或者HEAD, 倒是也会出现这种情况，但是反正只要配置default fallback，一样可以捕捉 / path的情况, 而且一般没人path分流会给 /, 根目录的情况直接就用default fallback了。
		failreason = 3
		return
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
		if last > 64 {
			last = 64
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
			return
		}
	}
	failreason = last //!isproxy时, 我们只判断了前64字节，如果访问url更长的话，这了还是会返回failreason的
	return

}
