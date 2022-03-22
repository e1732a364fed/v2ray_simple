package netLayer

import (
	"io"
	"log"
	"net"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
)

func GetRawConn(reader io.Reader) syscall.RawConn {
	if sc, ok := reader.(syscall.Conn); ok {
		rawConn, err := sc.SyscallConn()
		if err != nil {
			if utils.CanLogDebug() {
				log.Println("can't convert syscall.Conn to syscall.RawConn", reader, err)
			}
			return nil
		} else {
			return rawConn
		}
	}

	return nil
}

// 用于读端实现了 readv但是写端的情况，比如 从socks5读取 数据, 等裸协议的情况
// 小贴士：将该 net.Buffers 写入io.Writer的话，只需使用 其WriteTo方法。
/*
	使用方式

	var readConn, writeConn net.Conn
	//想办法初始化这两个Conn

	rawConn:= netLayer.GetRawConn(readConn)
	mr:=utils.GetReadVReader()
	buffers,err:= netLayer.ReadFromMultiReader(rawConn,mr)
	if err!=nil{
		log.Fatal("sfdfdaf")
	}
	buffers.WriteTo(writeConn)

	包装的代码见 TryCopy 函数; Relay函数也用到了

*/
func ReadFromMultiReader(rawReadConn syscall.RawConn, mr utils.MultiReader) (net.Buffers, error) {
	//v2ray里还使用了动态分配的方式，我们为了简便先啥也不做
	bs := make([][]byte, 16)
	// 实测16个buf已经完全够用，平时也就偶尔遇到5个buf的情况, 极速测速时会占用更多；
	// 16个1500那就是 24000, 23.4375 KB, 不算小了;
	for i := range bs {
		bs[i] = utils.GetMTU()
	}
	mr.Init(bs)

	var nBytes int32
	err := rawReadConn.Read(func(fd uintptr) bool {
		n := mr.Read(fd)
		if n < 0 {
			return false
		}

		nBytes = n
		return true
	})
	mr.Clear()
	if err != nil {
		ReleaseNetBuffers(bs)
		return nil, err
	}
	if nBytes == 0 {
		ReleaseNetBuffers(bs)
		return nil, io.EOF
	}

	//删减buffer 到合适的长度，并释放没用到的buf

	nBuf := 0
	for nBuf < len(bs) {
		if nBytes <= 0 {
			break
		}
		end := nBytes
		if end > int32(utils.StandardBytesLength) {
			end = int32(utils.StandardBytesLength)
		}
		bs[nBuf] = bs[nBuf][:end]
		nBytes -= end
		nBuf++
	}
	/*
		if utils.CanLogDebug() {
			// 可用于查看到底用了几个buf, 便于我们调整buf最大长度
			log.Println("release buf", len(bs)-nBuf)
		}
	*/
	ReleaseNetBuffers(bs[nBuf:])

	return bs[:nBuf], nil
}

func ReleaseNetBuffers(mb [][]byte) {
	for i := range mb {
		utils.PutBytes(mb[i])

	}
}
