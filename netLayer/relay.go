package netLayer

import (
	"io"
	"net"
	"reflect"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
	"go.uber.org/zap"
)

// TryCopy 尝试 循环 从 readConn 读取数据并写入 writeConn, 直到错误发生。
//会接连尝试 splice、循环readv 以及 原始Copy方法
func TryCopy(writeConn io.Writer, readConn io.Reader) (allnum int64, err error) {
	var multiWriter utils.MultiWriter

	var rawConn syscall.RawConn
	var isWriteConn_a_MultiWriter bool
	var isWriteConnBasic bool
	if ce := utils.CanLogDebug("TryCopy"); ce != nil {
		//log.Println("TryCopy", reflect.TypeOf(readConn), "->", reflect.TypeOf(writeConn))
		ce.Write(
			zap.String("from", reflect.TypeOf(readConn).String()),
			zap.String("->", reflect.TypeOf(writeConn).String()),
		)
	}

	if SystemCanSplice {

		rCanSplice := CanSpliceDirectly(readConn)

		if rCanSplice {
			var wCanSplice bool
			wCanSpliceDirectly := CanSpliceDirectly(writeConn)
			if wCanSpliceDirectly {
				wCanSplice = true
			} else {
				if CanSpliceEventually(writeConn) {
					wCanSplice = true
				}
			}

			if rCanSplice && wCanSplice {
				if ce := utils.CanLogDebug("copying with splice"); ce != nil {
					//log.Println("copying with splice")
					ce.Write()
				}

				goto copy
			}
		}

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

	if ce := utils.CanLogDebug("copying with readv"); ce != nil {
		//log.Println("copying with readv")
		ce.Write()
	}

	isWriteConnBasic = IsBasicConn(writeConn)

	if !isWriteConnBasic {
		multiWriter, isWriteConn_a_MultiWriter = writeConn.(utils.MultiWriter)
	}

	{
		var readv_mem *readvMem
		readv_mem = get_readvMem()
		defer put_readvMem(readv_mem)
		for {
			var buffers net.Buffers

			buffers, err = readvFrom(rawConn, readv_mem)
			if err != nil {
				return
			}
			var thisWriteNum int64
			var writeErr error

			// vless.UserConn 和 ws.Conn 实现了 utils.MultiWriter
			if isWriteConn_a_MultiWriter {
				thisWriteNum, writeErr = multiWriter.WriteBuffers(buffers)

			} else {
				// 这里不能直接使用 buffers.WriteTo, 因为它会修改buffer本身
				// 而我们为了缓存,是不能允许篡改的
				// 所以我们在确保 writeConn 不是 基本连接后, 要 自行write

				if isWriteConnBasic {
					//在basic时之所以可以 WriteTo，是因为它并不会用循环读取方式, 而是用底层的writev，
					// 而writev时是不会篡改 buffers的

					thisWriteNum, writeErr = buffers.WriteTo(writeConn)
				} else {

					thisWriteNum, writeErr = utils.BuffersWriteTo(buffers, writeConn)

				}
			}

			allnum += thisWriteNum
			if writeErr != nil {
				err = writeErr
				return
			}

			buffers = utils.RecoverBuffers(buffers, readv_buffer_allocLen, ReadvSingleBufLen)

		}
	}
classic:
	if ce := utils.CanLogDebug("copying with classic method"); ce != nil {
		//log.Println("copying with classic method")
		ce.Write()
	}
copy:

	//Copy内部实现 会自动进行splice, 若无splice实现则直接使用原始方法 “循环读取 并 写入”
	// 我们的 vless和 ws 的Conn均实现了ReadFrom方法，可以最终splice
	return io.Copy(writeConn, readConn)
}

// 类似TryCopy，但是只会读写一次; 因为只读写一次，所以没办法splice
func TryCopyOnce(writeConn io.Writer, readConn io.Reader) (allnum int64, err error) {
	var buffers net.Buffers
	var rawConn syscall.RawConn

	var rm *readvMem

	if ce := utils.CanLogDebug("TryCopy"); ce != nil {
		//log.Println("TryCopy", reflect.TypeOf(readConn), "->", reflect.TypeOf(writeConn))
		ce.Write(
			zap.String("from", reflect.TypeOf(readConn).String()),
			zap.String("->", reflect.TypeOf(writeConn).String()),
		)
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

	if ce := utils.CanLogDebug("copying with readv"); ce != nil {
		//log.Println("copying with readv")
		ce.Write()
	}

	rm = get_readvMem()
	defer put_readvMem(rm)

	buffers, err = readvFrom(rawConn, rm)
	if err != nil {
		return 0, err
	}
	allnum, err = utils.BuffersWriteTo(buffers, writeConn) //buffers.WriteTo(writeConn)

	return

classic:
	if ce := utils.CanLogDebug("copying with classic method"); ce != nil {
		//log.Println("copying with classic method")
		ce.Write()
	}

	bs := utils.GetPacket()
	n, e := readConn.Read(bs)
	if e != nil {
		utils.PutPacket(bs)
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
// 会自动优选 splice，readv，不行则使用经典拷贝. 拷贝完成后会主动关闭双方连接.
func Relay(realTargetAddr *Addr, wrc, wlc io.ReadWriteCloser) {

	if ce := utils.CanLogDebug("转发结束"); ce != nil {
		go func() {
			n, e := TryCopy(wrc, wlc)
			//log.Println("本地->远程 转发结束", realTargetAddr.String(), n, e)
			ce.Write(zap.String("direction", "本地->远程"),
				zap.String("target", realTargetAddr.String()),
				zap.Int64("copied bytes", n),
				zap.Error(e),
			)

			wlc.Close()
			wrc.Close()

		}()

		n, e := TryCopy(wlc, wrc)
		//log.Println("远程->本地 转发结束", realTargetAddr.String(), n, e)

		ce.Write(zap.String("direction", "远程->本地"),
			zap.String("target", realTargetAddr.String()),
			zap.Int64("copied bytes", n),
			zap.Error(e),
		)

		wlc.Close()
		wrc.Close()
	} else {
		go func() {
			TryCopy(wrc, wlc)

			wlc.Close()
			wrc.Close()

		}()

		TryCopy(wlc, wrc)

		wlc.Close()
		wrc.Close()
	}

	//log.Println("copy called", count)
	//count++
}

//var count int
