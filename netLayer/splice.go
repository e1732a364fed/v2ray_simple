package netLayer

import (
	"errors"
	"io"
	"net"
	"runtime"

	"github.com/hahahrfool/v2ray_simple/utils"
)

var errInvalidWrite = errors.New("vless readfrom, invalid write result")

const SystemCanSplice = runtime.GOARCH != "wasm" && runtime.GOOS != "windows"

type Splicer interface {
	EverPossibleToSplice() bool  //是否有机会splice, 如果这个返回false，则永远无法splice; 主要审视自己能否向裸连接写入数据; 读不用splicer担心。
	CanSplice() (bool, net.Conn) //当前状态是否可以splice
}

//这里认为能 splice 或 sendfile的 都算，具体可参考go标准代码的实现, 总之就是tcp和 unix domain socket 可以.
// 若不是基本Conn，则会试图转换为Splicer并获取底层Conn
func CanSpliceDirectly(r any) bool {

	if _, ok := r.(*net.TCPConn); ok {
		return true
	} else if _, ok := r.(*net.UnixConn); ok {

		return true
	}
	return false

}

func CanSpliceEventually(r any) bool {
	if s, ok := r.(Splicer); ok {
		return s.EverPossibleToSplice()
	}
	return false
}

//从r读取数据，写入 maySpliceConn 或者 classicWriter, 在条件合适时会使用splice进行加速。
// 本函数主要应用于裸奔时，一端是socks5/直连,另一端是vless/vless+ws的情况, 因为vless等协议就算裸奔也是要处理一下数据头等情况的, 所以需要进行处理才可裸奔.
//
// 注意，splice只有在 maySpliceConn【本身是】或者【变成】 basicConn， 且 r 也是 basicConn时，才会发生。
// 如果r本身就不是 basicConn，则调用本函数没有意义, 因为既然拿不到basicConn那就不是裸奔，也就不可能splice。
func TryReadFrom_withSplice(classicWriter io.Writer, maySpliceConn net.Conn, r io.Reader, canDirectFunc func() bool) (written int64, err error) {
	//log.Println("TryReadFrom_withSplice called")

	underlay_canSpliceDirectly := CanSpliceDirectly(maySpliceConn)

	var underlay_canSpliceEventually bool

	if !underlay_canSpliceDirectly {
		underlay_canSpliceEventually = CanSpliceEventually(maySpliceConn)
	}

	var splicer Splicer

	if underlay_canSpliceEventually {
		splicer = maySpliceConn.(Splicer)
	}

	/*
		分多钟情况，
		1. underlay直接是基础连接（underlay_canSpliceDirectly），且现在直接 canDirectFunc 就是true, 此时直接 splice
		2. underlay直接是基础连接（underlay_canSpliceDirectly），但现在的连接阶段还不能直接直连，此时要读写一次然后判断一次，直到 canDirectFunc 变成 true
		3. underlay 不是基础连接，但是 是 Splicer（underlay_canSpliceEventually），且此时我们先等待 underlay已经处于 可直连状态, 即 splicer.CanSplice()变成 true，然后再确保 canDirectFunc 返回true
		4. underlay啥也不是，直接经典拷贝。
	*/

	if underlay_canSpliceDirectly || underlay_canSpliceEventually {
		if underlay_canSpliceDirectly && canDirectFunc() {
			if rt, ok := maySpliceConn.(io.ReaderFrom); ok {
				return rt.ReadFrom(r)
			} else {
				panic("uc.underlayIsBasic, but can't cast to ReadFrom")
			}
		} else {
			//循环读写，直到 canDirectFunc 和 splicer.CanSplice() 都为true

			buf := utils.GetPacket()
			defer utils.PutPacket(buf)
			for {
				nr, er := r.Read(buf)
				if nr > 0 {
					nw, ew := classicWriter.Write(buf[0:nr])
					if nw < 0 || nr < nw {
						nw = 0
						if ew == nil {
							ew = errInvalidWrite
						}
					}
					written += int64(nw)
					if ew != nil {
						err = ew
						break
					}
					if nr != nw {
						err = io.ErrShortWrite
						break
					}
				}
				if er != nil {
					if er != io.EOF {
						err = er
					}
					break
				}

				if underlay_canSpliceDirectly {
					if rt, ok := maySpliceConn.(io.ReaderFrom); ok {
						var readfromN int64
						readfromN, err = rt.ReadFrom(r)
						written += readfromN
						return
					} else {
						panic("uc.underlayIsBasic, but can't cast to ReadFrom")
					}
				} else {

					if canStartSplice, rawConn := splicer.CanSplice(); canStartSplice {
						if rt, ok := rawConn.(io.ReaderFrom); ok {
							var readfromN int64
							readfromN, err = rt.ReadFrom(r)
							written += readfromN
							return
						} else {
							panic("uc.underlayIsBasic, but can't cast to ReadFrom")
						}
					}

				}

			} //for read
			return

		} //cant direct write

	} else { //splice not possible, 仅仅循环读写即可

		return ClassicReadFrom(classicWriter, r)
	}

}

func ClassicReadFrom(w io.Writer, r io.Reader) (written int64, err error) {
	buf := utils.GetPacket()
	defer utils.PutPacket(buf)
	for {
		nr, er := r.Read(buf)
		if nr > 0 {
			nw, ew := w.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errInvalidWrite
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return
}
