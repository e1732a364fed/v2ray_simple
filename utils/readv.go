package utils

import "sync"

var (
	// readv pool, 缓存 mr和buffers，进一步减轻内存分配负担
	readvPool sync.Pool
)

const (
	Readv_buffer_allocLen = 8
	ReadvSingleBufLen     = 4096
)

func init() {

	readvPool = sync.Pool{
		New: newReadvMem,
	}

	//预先放进去两个

	readvPool.Put(newReadvMem())
	readvPool.Put(newReadvMem())
}

// 缓存 ReadvMem 以及对应分配的系统相关的 SystemReadver.
// 使用 ReadvMem的最大好处就是 buffers 和 mr 都是不需要 释放的.
//
//	因为不需释放mr, 所以也就节省了多次 mr.Init 的开销.
//
// 该 ReadvMem 以及 readvPool 专门服务于 TryCopy 函数.
type ReadvMem struct {
	Buffers [][]byte
	Mr      SystemReadver
}

func allocReadvBuffers(mr SystemReadver, len int) [][]byte {
	bs := make([][]byte, len)

	for i := range bs {
		// 这里单独make，而不是从 其它pool中获取, 这样可以做到专用
		bs[i] = make([]byte, ReadvSingleBufLen)
	}
	mr.Init(bs, ReadvSingleBufLen)
	return bs
}

func newReadvMem() any {
	mr := GetReadVReader()
	return &ReadvMem{
		Mr:      mr,
		Buffers: allocReadvBuffers(mr, Readv_buffer_allocLen),
	}
}

func Get_readvMem() *ReadvMem {
	return readvPool.Get().(*ReadvMem)

}

// 将创建好的rm放回 readvPool
func Put_readvMem(rm *ReadvMem) {
	rm.Buffers = RecoverBuffers(rm.Buffers, Readv_buffer_allocLen, ReadvSingleBufLen)
	readvPool.Put(rm)
}
