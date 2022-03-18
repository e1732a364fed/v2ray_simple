/*
Package httpLayer 提供http层的一些方法和定义，比如fallback
*/
package httpLayer

//从一个字节串中试图获取 http请求的 path, 空字节表示获取失败
func GetRequestPATH_from_Bytes(bs []byte) string {
	goto check
no:
	return ""

check:

	if len(bs) < 18 {
		goto no
	}

	if bs[4] == '*' {
		goto no //not h2c

	}
	//http 方法有：GET, POST, HEAD, PUT, DELETE, OPTIONS, CONNECT, PRI

	//看 v2ray的实现，似乎用了循环的方法，从 5 -> 9字节读取，实际上就是考虑到 http方法可以是 3-7字节
	// v2ray的循环是可能有问题的，还是先过滤掉非http方法比较好，否则回落的话，多了一个转换path成字符串的开销
	//我们使用另一种方法，不使用循环 过滤 http方法

	//3字节， Get，Put，（pri被过滤了）
	// 4字节，Post,Head,
	// 5字节
	// 6字节， delete，
	// 7字节，options，connect

	if bs[5] == ' ' {
		goto no
	}

	shouldSpaceIndex := 0

	switch bs[0] {
	case 'G':
		if bs[1] == 'E' && bs[2] == 'T' {
			shouldSpaceIndex = 3
		}
	case 'P':
		if bs[1] == 'U' && bs[2] == 'T' {
			shouldSpaceIndex = 3
		} else if bs[1] == 'O' && bs[2] == 'S' && bs[3] == 'T' {
			shouldSpaceIndex = 4
		}
	case 'H':
		if bs[1] == 'E' && bs[2] == 'A' && bs[3] == 'D' {
			shouldSpaceIndex = 4
		}
	case 'D':
		if bs[1] == 'E' && bs[2] == 'L' && bs[3] == 'E' && bs[4] == 'T' && bs[5] == 'E' {
			shouldSpaceIndex = 6
		}
	case 'O':
		if bs[1] == 'P' && bs[2] == 'T' && bs[3] == 'I' && bs[4] == 'O' && bs[5] == 'N' && bs[6] == 'S' {
			shouldSpaceIndex = 7
		}
	case 'C':
		if bs[1] == 'O' && bs[2] == 'N' && bs[3] == 'N' && bs[4] == 'E' && bs[5] == 'C' && bs[6] == 'T' {
			shouldSpaceIndex = 7
		}
	}

	if shouldSpaceIndex == 0 || bs[shouldSpaceIndex] != ' ' {
		goto no
	}
	shouldSlashIndex := shouldSpaceIndex + 1

	if bs[shouldSlashIndex] != '/' {
		goto no
	}

	//一般请求样式类似 GET /sdfdsffs.html HTTP/1.1
	//所以找到第二个空格的位置即可，

	for i := shouldSlashIndex; i < shouldSlashIndex+64; i++ {
		b := bs[i]
		if b == '\r' || b == '\n' {
			break
		}
		if b == ' ' {
			return string(bs[shouldSlashIndex:i])
		}
	}
	goto no

}
