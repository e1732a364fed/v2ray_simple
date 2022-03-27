package utils

import (
	"io"
	"log"
)

//一种简单的组合
// 在ws包中被用到.
type RW struct {
	io.Reader
	io.Writer
}

// SystemReadver 是平台相关的 用于 调用readv的 工具.
// 该 SystemReadver 的用例请参照 netLayer.readvFrom , 在 netLayer/readv.go中;
// SystemReadver的具体平台相关的实现见 readv_*.go; 用 GetReadVReader() 函数来获取本平台的对应实现。
type SystemReadver interface {
	Init(bs [][]byte, singleBufLen int) //将 给出的buffer 放入内部实际数据中
	Read(fd uintptr) (uint32, error)    //读取一次文件，并放入 buffer中
	Clear()                             //清理内部buffer
}

// 因为 net.Buffers 的 WriteTo方法只会查看其是否实现了net包私有的 writeBuffers 接口
// 我们无法用WriteTo来给其它 代码提升性能；因此我们重新定义一个新的借口, 实现了 MultiWriter
// 接口的结构 我们就认为它会提升性能，比直接用 net.Buffers.WriteTo 要更强.
/*
	本接口 在代理中的用途，基本上只适合 加密层 能够做到分组加密 的情况; 因为如果不加密的话就是裸协议，直接就splice或者writev了,也不需要这么麻烦;

	如果是tls的话，可能涉及自己魔改tls把私有函数暴露出来然后分组加密；

	如果是vmess的话，倒是有可能的，不过我还没研究vmess的 加密细节；

	而如果是ss 那种简单混淆 异或加密的话，则是完全可以的

	分组加密然后 一起用 writev 发送出去，可以降低网络延迟, 不过writev性能的提升可能是非常细微的, 也不必纠结这里.

	如果考虑另一种情况，即需要加包头和包尾，则区别就很大了；

	WriteTo会调用N次Write，如果包装的话，会包装N 个包头和 包尾；而如果我们实现WriteBuffers方法，
	只需先写入包头，而在 最后一个 []byte 后加 包尾，那么就可以获得性能提升,
	我们只需增添两个新的 []byte 放在其前后即可, 然后再用 writev 一起发送出去

	那么实际上 websocket 的gobwas/ws 包在不开启缓存时，就是 每次Write都写一次包头的情况;

	所以websocket很有必要实现 WriteBuffers 方法.

	目前实现 的有 vless.UserConn 和  ws.Conn
*/
type MultiWriter interface {
	WriteBuffers([][]byte) (int64, error)
}

//获取所有子[]byte 长度总和
func BuffersLen(bs [][]byte) (allnum int) {
	if len(bs) < 1 {
		return 0
	}
	for _, b := range bs {
		allnum += len(b)
	}
	return allnum
}

func PrintBuffers(bs [][]byte) {
	for i, b := range bs {
		log.Println(i, b)
	}
}

//削减buffer内部的子[]byte 到合适的长度;返回削减后 bs应有的长度.
func ShrinkBuffers(bs [][]byte, all_len int, SingleBufLen int) int {
	curIndex := 0
	for curIndex < len(bs) {
		if all_len <= 0 {
			break
		}
		end := all_len
		if end > SingleBufLen {
			end = SingleBufLen
		}
		bs[curIndex] = bs[curIndex][:end]
		all_len -= end
		curIndex++
	}
	return curIndex
}

//通过reslice 方式将 bs的长度以及 子 []byte 的长度 恢复至指定长度
func RecoverBuffers(bs [][]byte, oldLen, old_sub_len int) [][]byte {
	bs = bs[:oldLen]
	for i, v := range bs {
		bs[i] = v[:old_sub_len]
	}
	return bs
}

func BuffersWriteTo(bs [][]byte, writeConn io.Writer) (num int64, err2 error) {
	for _, b := range bs {
		nb, err := writeConn.Write(b)
		num += int64(nb)
		if err != nil {

			err2 = err
			break
		}
	}
	return
}

// 如果 分配了新内存来 包含数据，则 duplicate ==true; 如果利用了原有的第一个[]byte, 则 duplicate==false
// 如果 duplicate==false, 不要 使用 PutPacket等方法放入Pool；
//  因为 在更上级的调用会试图去把 整个bs 放入pool;
func MergeBuffers(bs [][]byte) (result []byte, duplicate bool) {
	if len(bs) < 1 {
		return
	}
	b0 := bs[0]
	if len(bs) == 1 {
		return b0, false
	}
	allLen := BuffersLen(bs)

	if allLen <= cap(b0) { //所有的长度 小于第一个的cap，那么可以全放入第一个中;实际readv不会出现这种情况
		b0 = b0[:allLen]
		cursor := len(b0)
		for i := 1; i < len(bs); i++ {
			cursor += copy(b0[cursor:], bs[i])
		}
		return b0, false
	}

	if allLen <= MaxBufLen {
		result = GetPacket()

	} else {
		result = make([]byte, allLen) //实际目前的readv实现也很难出现这种情况, 默认16个1500是不会出这种情况的
	}

	cursor := 0
	for i := 0; i < len(bs); i++ {
		cursor += copy(result[cursor:], bs[i])
	}

	return result[:allLen], true
}
