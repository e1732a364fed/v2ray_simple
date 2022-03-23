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

//这里认为能 splice 或 sendfile的 都算，具体可参考go标准代码的实现, 总之就是tcp和uds可以
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
	var isWriteConn_a_MultiWriter bool
	var multiWriter utils.MultiWriter
	isWriteConnBasic := IsBasicConn(writeConn)

	if !isWriteConnBasic {
		multiWriter, isWriteConn_a_MultiWriter = writeConn.(utils.MultiWriter)
	}

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
	defer mr.Clear()
	defer utils.ReleaseBuffers(buffers, readv_buffer_allocLen)

	for {
		buffers, err = ReadFromMultiReader(rawConn, mr, buffers)
		if err != nil {
			return 0, err
		}
		var num int64
		var err2 error

		//如vless协议，肯定走这里，因为 vless.UserConn 实现了 utils.MultiWriter
		if isWriteConn_a_MultiWriter {
			num, err2 = multiWriter.WriteBuffers(buffers)

		} else {
			num, err2 = buffers.WriteTo(writeConn)
		}

		allnum += num
		if err2 != nil {
			err = err2
			return
		}

		buffers = utils.RecoverBuffers(buffers, 16, utils.StandardBytesLength)

	}
classic:
	if utils.CanLogDebug() {
		log.Println("copying with classic method")
	}
copy:

	//Copy内部实现 会自动进行splice, 若无splice实现则直接使用原始方法 “循环读取 并 写入”
	return io.Copy(writeConn, readConn)
}

// 类似TryCopy，但是只会读写一次; 因为只读写一次，所以没办法splice
func TryCopyOnce(writeConn io.Writer, readConn io.Reader) (allnum int64, err error) {
	var mr utils.MultiReader
	var buffers net.Buffers
	var rawConn syscall.RawConn

	if utils.CanLogDebug() {
		log.Println("TryCopy", reflect.TypeOf(readConn), "->", reflect.TypeOf(writeConn))
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
	defer mr.Clear()
	defer utils.ReleaseBuffers(buffers, 16)

	buffers, err = ReadFromMultiReader(rawConn, mr, nil)
	if err != nil {
		return 0, err
	}
	allnum, err = buffers.WriteTo(writeConn)

	return

classic:
	if utils.CanLogDebug() {
		log.Println("copying with classic method")
	}

	bs := utils.GetPacket()
	n, e := readConn.Read(bs)
	if e != nil {
		return 0, e
	}
	n, e = writeConn.Write(bs[:n])
	utils.PutPacket(bs)
	return int64(n), e
}

// 从conn1读取 写入到 conn2，并同时从 conn2读取写入conn1
// 阻塞
// 返回从 conn1读取 写入到 conn2的数据
// UseReadv==true 时 内部使用 TryCopy 进行拷贝
// 会自动优选 splice，readv，不行则使用经典拷贝
func Relay(conn1, conn2 io.ReadWriter) (int64, error) {

	if UseReadv {
		go TryCopy(conn1, conn2)
		return TryCopy(conn2, conn1)

	} else {
		go io.Copy(conn1, conn2)
		return io.Copy(conn2, conn1)
	}
}
