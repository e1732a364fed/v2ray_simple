package netLayer

import (
	"flag"
	"io"
	"log"
	"net"
	"reflect"
	"runtime"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
)

const SystemCanSplice = runtime.GOARCH != "wasm" && runtime.GOOS != "windows"

var UseReadv bool

func init() {
	flag.BoolVar(&UseReadv, "readv", true, "toggle the use of 'readv' syscall")

}

//这里认为能 splice 或 sendfile的 都算
func CanSplice(r interface{}) bool {

	if _, ok := r.(*net.TCPConn); ok {
		return true
	} else if _, ok := r.(*net.UnixConn); ok {

		return true
	}
	return false

}

// TryCopy 尝试 循环 从 readConn 读取数据并写入 writeConn, 直到错误发生。
//会接连尝试 splice、循环readv 以及 原始Copy方法
func TryCopy(writeConn io.Writer, readConn io.Reader) (allnum int64, err error) {
	var mr utils.MultiReader
	var buffers net.Buffers
	var rawConn syscall.RawConn

	if utils.CanLogDebug() {
		log.Println("TryCopy", reflect.TypeOf(readConn), "->", reflect.TypeOf(writeConn))
	}

	if SystemCanSplice && CanSplice(readConn) && CanSplice(writeConn) {
		if utils.CanLogDebug() {
			log.Println("copying with splice")
		}
		goto copy
	}
	// 不全 支持splice的话，我们就考虑 read端 可 readv 的情况
	// 连readv都不让 那就直接 经典拷贝
	if !UseReadv {
		goto classic
	}

	rawConn = GetRawConn(readConn)

	if rawConn == nil {
		goto classic
	}

	mr = utils.GetReadVReader()
	if utils.CanLogDebug() {
		log.Println("copying with readv")
	}
	for {
		buffers, err = ReadFromMultiReader(rawConn, mr)
		if err != nil {
			return 0, err
		}
		num, err2 := buffers.WriteTo(writeConn)
		allnum += num
		if err2 != nil {
			err = err2
			return
		}
		ReleaseNetBuffers(buffers)
	}
classic:
	if utils.CanLogDebug() {
		log.Println("copying with classic method")
	}
copy:

	//Copy内部实现 会自动进行splice, 若无splice实现则直接使用原始方法 “循环读取 并 写入”
	return io.Copy(writeConn, readConn)
}

// 类似TryCopy，但是只会读写一次
func TryCopyOnce(writeConn io.Writer, readConn io.Reader) (allnum int64, err error) {
	var mr utils.MultiReader
	var buffers net.Buffers
	var rawConn syscall.RawConn

	if utils.CanLogDebug() {
		log.Println("TryCopy", reflect.TypeOf(readConn), "->", reflect.TypeOf(writeConn))
	}

	if SystemCanSplice && CanSplice(readConn) && CanSplice(writeConn) {
		if utils.CanLogDebug() {
			log.Println("copying with splice")
		}
		goto copy
	}
	// 不全 支持splice的话，我们就考虑 read端 可 readv 的情况
	// 连readv都不让 那就直接 经典拷贝
	if !UseReadv {
		goto classic
	}

	rawConn = GetRawConn(readConn)

	if rawConn == nil {
		goto classic
	}

	mr = utils.GetReadVReader()
	if utils.CanLogDebug() {
		log.Println("copying with readv")
	}

	buffers, err = ReadFromMultiReader(rawConn, mr)
	if err != nil {
		return 0, err
	}
	allnum, err = buffers.WriteTo(writeConn)

	ReleaseNetBuffers(buffers)
	return

classic:
	if utils.CanLogDebug() {
		log.Println("copying with classic method")
	}
copy:

	//Copy内部实现 会自动进行splice, 若无splice实现则直接使用原始方法 “循环读取 并 写入”
	return io.Copy(writeConn, readConn)
}

// 从conn1读取 写入到 conn2，并同时从 conn2读取写入conn1
// 阻塞
// 返回从 conn1读取 写入到 conn2的数据
// UseReadv==true 时 内部使用 TryCopy 进行拷贝
func Relay(conn1, conn2 io.ReadWriter) (int64, error) {

	if UseReadv {
		go TryCopy(conn1, conn2)
		return TryCopy(conn2, conn1)

	} else {
		go io.Copy(conn1, conn2)
		return io.Copy(conn2, conn1)
	}
}

// 阻塞.
func RelayUDP(putter UDP_Putter, extractor UDP_Extractor) {

	go func() {
		for {
			raddr, bs, err := extractor.GetNewUDPRequest()
			if err != nil {
				break
			}
			err = putter.WriteUDPRequest(raddr, bs)
			if err != nil {
				break
			}
		}
	}()

	for {
		raddr, bs, err := putter.GetNewUDPResponse()
		if err != nil {
			break
		}
		err = extractor.WriteUDPResponse(raddr, bs)
		if err != nil {
			break
		}
	}
}