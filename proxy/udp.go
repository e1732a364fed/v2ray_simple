package proxy

import (
	"io"
	"net"
)

const (
	MaxUDP_packetLen = 64 * 1024 // 关于 udp包数据长度，可参考 https://cloud.tencent.com/developer/article/1021196
)

//////////////////// 接口 ////////////////////

type UDPRequestReader interface {
	GetNewUDPRequest() (*net.UDPAddr, []byte, error)
}

type UDPResponseWriter interface {
	WriteUDPResponse(*net.UDPAddr, []byte) error
}

// UDP_Extractor, 用于从一个虚拟的协议中提取出 udp请求
//
// 从一个未知协议中读取UDP请求，然后试图得到该请求的回应(大概率是直接通过direct发出) 并写回
type UDP_Extractor interface {
	UDPRequestReader
	UDPResponseWriter
}

// 写入一个UDP请求
type UDPRequestWriter interface {
	WriteUDPRequest(*net.UDPAddr, []byte) error
}

//拉取一个新的 UDP 响应
type UDPResponseReader interface {
	GetNewUDPResponse() (*net.UDPAddr, []byte, error)
}

// UDP_Putter, 用于把 udp请求转换成 虚拟的协议
//
// 向一个特定的协议 写入 UDP请求，然后试图读取 该请求的回应
type UDP_Putter interface {
	UDPRequestWriter
	UDPResponseReader
}

//////////////////// 具体实现 ////////////////////

// 最简单的 UDP_Putter，用于客户端; 不处理内部数据，直接认为要 发送给服务端的信息 要发送到一个特定的地址
//	如果指定的地址不是 默认的地址，则发送到 unknownRemoteAddrMsgWriter
//
// 对于 vless v1来说, unknownRemoteAddrMsgWriter 要做的 就是 新建一个与服务端的 请求udp的连接，
//  然后这个新连接就变成了新的 UniUDP_Putter
type UniUDP_Putter struct {
	targetAddr *net.UDPAddr
	io.ReadWriter

	unknownRemoteAddrMsgWriter UDPRequestWriter
}

//
func (e *UniUDP_Putter) GetNewUDPResponse() (*net.UDPAddr, []byte, error) {
	bs := make([]byte, MaxUDP_packetLen)
	n, err := e.ReadWriter.Read(bs)
	if err != nil {
		return e.targetAddr, nil, err
	}
	return e.targetAddr, bs[:n], nil
}

func (e *UniUDP_Putter) WriteUDPRequest(addr *net.UDPAddr, bs []byte) (err error) {

	if addr.String() == e.targetAddr.String() {
		_, err = e.ReadWriter.Write(bs)

		return
	} else {
		if e.unknownRemoteAddrMsgWriter == nil {
			return
		}

		return e.unknownRemoteAddrMsgWriter.WriteUDPRequest(addr, bs)
	}

}

// 最简单的 UDP_Extractor，用于服务端; 不处理内部数据，直接认为客户端传来的内部数据的目标为一个特定值。
//	收到的响应数据的来源 如果和 targetAddr 相同的话，直接写入传入的 ReadWriter
//  收到的外界数据的来源 如果和 targetAddr 不同的话，那肯定就是使用了fullcone，那么要传入 unknownRemoteAddrMsgWriter； 如果New时传入unknownRemoteAddrMsgWriter的 是nil的话，那么意思就是不支持fullcone，将直接舍弃这一部分数据。
type UniUDP_Extractor struct {
	targetAddr *net.UDPAddr
	io.ReadWriter

	unknownRemoteAddrMsgWriter UDPResponseWriter
}

// 新建，unknownRemoteAddrMsgWriter 用于写入 未知来源响应，rw 用于普通的客户请求的目标的响应
func NewUniUDP_Extractor(addr *net.UDPAddr, rw io.ReadWriter, unknownRemoteAddrMsgWriter UDPResponseWriter) *UniUDP_Extractor {
	return &UniUDP_Extractor{
		targetAddr:                 addr,
		ReadWriter:                 rw,
		unknownRemoteAddrMsgWriter: unknownRemoteAddrMsgWriter,
	}
}

// 从客户端连接中 提取出 它的 UDP请求，就是直接读取数据。然后搭配上之前设置好的地址
func (e *UniUDP_Extractor) GetNewUDPRequest() (*net.UDPAddr, []byte, error) {
	bs := make([]byte, MaxUDP_packetLen)
	n, err := e.ReadWriter.Read(bs)
	if err != nil {
		return e.targetAddr, nil, err
	}
	return e.targetAddr, bs[:n], nil
}

// WriteUDPResponse 写入远程服务器的响应；要分情况讨论。
// 因为是单一目标extractor，所以正常情况下 传入的response 的源地址 也 应和 e.targetAddr 相同，
//  如果地址不同的话，那肯定就是使用了fullcone，那么要传入 unknownRemoteAddrMsgWriter
func (e *UniUDP_Extractor) WriteUDPResponse(addr *net.UDPAddr, bs []byte) (err error) {

	if addr.String() == e.targetAddr.String() {
		_, err = e.ReadWriter.Write(bs)

		return
	} else {
		//如果未配置 unknownRemoteAddrMsgWriter， 则说明不支持fullcone。这并不是错误，而是可选的。看你想不想要fullcone
		if e.unknownRemoteAddrMsgWriter == nil {
			return
		}

		return e.unknownRemoteAddrMsgWriter.WriteUDPResponse(addr, bs)
	}

}

type UDPAddrData struct {
	Addr *net.UDPAddr
	Data []byte
}

//一种简单的本地 UDP_Extractor + UDP_Putter
type UDP_Pipe struct {
	requestChan, responseChan chan UDPAddrData
}

func NewUDP_Pipe() *UDP_Pipe {
	return &UDP_Pipe{
		requestChan:  make(chan UDPAddrData, 10),
		responseChan: make(chan UDPAddrData, 10),
	}
}
func (u *UDP_Pipe) GetNewUDPRequest() (*net.UDPAddr, []byte, error) {
	d := <-u.requestChan
	return d.Addr, d.Data, nil

}

func (u *UDP_Pipe) GetNewUDPResponse() (*net.UDPAddr, []byte, error) {
	d := <-u.responseChan
	return d.Addr, d.Data, nil

}

// 会保存bs的副本，不必担心数据被改变的问题。
func (u *UDP_Pipe) WriteUDPResponse(addr *net.UDPAddr, bs []byte) error {
	bsCopy := make([]byte, len(bs))
	copy(bsCopy, bs)

	u.responseChan <- UDPAddrData{
		Addr: addr,
		Data: bsCopy,
	}
	return nil
}

// 会保存bs的副本，不必担心数据被改变的问题。
func (u *UDP_Pipe) WriteUDPRequest(addr *net.UDPAddr, bs []byte) error {
	bsCopy := make([]byte, len(bs))
	copy(bsCopy, bs)

	u.requestChan <- UDPAddrData{
		Addr: addr,
		Data: bsCopy,
	}
	return nil
}
