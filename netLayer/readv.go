package netLayer

import (
	"flag"
	"io"
	"sync"
	"syscall"

	"github.com/hahahrfool/v2ray_simple/utils"
)

//v2ray里还使用了动态分配的方式，我们为了简便就固定长度. 另外v2ray单个缓存长度是是2k，我们单个是MTU.
// 实测16个buf已经完全够用，平时也就偶尔遇到5个buf的情况, 极速测速时会占用更多；
// 16个1500那就是 24000, 23.4375 KB, 不算小了;
// 而且我们还使用了Pool来进行缓存,减轻了内存负担, 所以也没必要为了解决内存分配而频繁调整长度.

//经过测试，网速越快、延迟越小，越不需要readv, 此时首包buf越大越好, 因为一次系统底层读取就会读到一大块数据, 此时再用readv分散写入 实际上就是反效果; readv的数量则不需要太多

//在内网单机自己连自己测速时,readv会导致降速.

const readv_buffer_allocLen = 8
const ReadvSingleBufLen = 4096

var (
	// readv pool, 缓存 mr和buffers，进一步减轻内存分配负担
	readvPool sync.Pool

	UseReadv bool
)

func init() {
	flag.BoolVar(&UseReadv, "readv", true, "toggle the use of 'readv' syscall")

	readvPool = sync.Pool{
		New: newReadvMem,
	}

	//预先放进去两个

	readvPool.Put(newReadvMem())
	readvPool.Put(newReadvMem())
}

// 缓存 readvMem 以及对应分配的系统相关的 utils.SystemReadver.
// 使用 readvMem的最大好处就是 buffers 和 mr 都是不需要 释放的.
//  因为不需释放mr, 所以也就节省了多次 mr.Init 的开销.
// 该 readvMem 以及 readvPool 专门服务于 TryCopy 函数.
type readvMem struct {
	buffers [][]byte
	mr      utils.SystemReadver
}

func allocReadvBuffers(mr utils.SystemReadver, len int) [][]byte {
	bs := make([][]byte, len)

	for i := range bs {
		// 这里单独make，而不是从 其它pool中获取, 这样可以做到专用
		bs[i] = make([]byte, ReadvSingleBufLen)
	}
	mr.Init(bs, ReadvSingleBufLen)
	return bs
}

func newReadvMem() any {
	mr := utils.GetReadVReader()
	return &readvMem{
		mr:      mr,
		buffers: allocReadvBuffers(mr, readv_buffer_allocLen),
	}
}

func get_readvMem() *readvMem {
	return readvPool.Get().(*readvMem)
}

//将创建好的rm放回 readvPool
func put_readvMem(rm *readvMem) {
	rm.buffers = utils.RecoverBuffers(rm.buffers, readv_buffer_allocLen, ReadvSingleBufLen)
	readvPool.Put(rm)
}

/*
//目前我们使用内存池，所以暂时不需要手动释放内存，
//如果真需要手动申请、释放，可参考  https://studygolang.com/articles/9721
func (rm *readvMem) destroy() {
	rm.mr.Clear()
	rm.mr = nil
	rm.buffers = nil
}*/

/* readvFrom 用于读端实现了 readv但是写端的情况，比如 从socks5读取 数据, 等裸协议的情况。

若allocedBuffers未给出，会使用 utils.AllocMTUBuffers 来初始化 缓存。

返回错误时，依然会返回 原buffer 或者 在函数内部新分配的buffer. 本函数不负责 释放分配的内存. 因为有时需要重复利用缓存。

小贴士：将该 [][]byte 写入io.Writer的话，只需使用 其WriteTo方法, 即可自动适配writev。

TryCopy函数使用到了本函数 来进行readv相关操作。
*/
func readvFrom(rawReadConn syscall.RawConn, rm *readvMem) ([][]byte, error) {

	if rm == nil {
		rm = get_readvMem() //utils.AllocMTUBuffers(mr, readv_buffer_allocLen)
	}

	allocedBuffers := rm.buffers

	var nBytes uint32
	err := rawReadConn.Read(func(fd uintptr) bool {
		n, e := rm.mr.Read(fd)
		if e != nil {
			return false
		}

		nBytes = n
		return true
	})
	if err != nil {

		return allocedBuffers, err
	}
	if nBytes == 0 {
		return allocedBuffers, io.EOF
	}

	nBuf := utils.ShrinkBuffers(allocedBuffers, int(nBytes), ReadvSingleBufLen)
	/*
		if utils.CanLogDebug() {
			// 可用于查看到底用了几个buf, 便于我们调整buf最大长度
			log.Println("release buf", len(allocedBuffers)-nBuf)
		}
	*/

	return allocedBuffers[:nBuf], nil
}
