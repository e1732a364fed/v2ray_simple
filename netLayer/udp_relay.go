package netLayer

import (
	"io"
	"net"
	"sync"
	"time"
)

const (
	MaxUDP_packetLen = 64 * 1024 // 关于 udp包数据长度，可参考 https://cloud.tencent.com/developer/article/1021196

	UDP_timeout = time.Hour * 2 //udp不能无限监听, 否则每一个udp申请都对应一个
)

//本文件内含 一些 转发 udp 数据的 接口与方法

// 阻塞.
func RelayUDP(putter UDP_Putter, extractor UDP_Extractor, dialFunc func(targetAddr Addr) (io.ReadWriter, error)) {

	go func() {
		for {
			raddr, bs, err := extractor.GetNewUDPRequest()
			if err != nil {
				break
			}
			err = putter.WriteUDPRequest(raddr, bs, dialFunc)
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

//////////////////// 接口 ////////////////////

type UDPRequestReader interface {
	GetNewUDPRequest() (net.UDPAddr, []byte, error)
}

type UDPResponseWriter interface {
	WriteUDPResponse(net.UDPAddr, []byte) error
}

// UDP_Extractor, 用于从一个虚拟的协议中提取出 udp请求
//
// 从一个未知协议中读取UDP请求，然后试图得到该请求的回应(大概率是直接通过direct发出) 并写回
type UDP_Extractor interface {
	UDPRequestReader
	UDPResponseWriter
}

// 写入一个UDP请求; 可以包裹成任意协议。
// 因为有时该地址从来没申请过，所以此时就要用dialFunc创建一个新连接
type UDPRequestWriter interface {
	WriteUDPRequest(target net.UDPAddr, request []byte, dialFunc func(targetAddr Addr) (io.ReadWriter, error)) error
	CloseUDPRequestWriter() //如果read端失败,则一定需要close Write端. CloseUDPRequestWriter就是这个用途.
}

//拉取一个新的 UDP 响应
type UDPResponseReader interface {
	GetNewUDPResponse() (net.UDPAddr, []byte, error)
}

// UDP_Putter, 用于把 udp请求转换成 虚拟的协议
//
// 向一个特定的协议 写入 UDP请求，然后试图读取 该请求的回应. 比如vless.Client就实现了它
type UDP_Putter interface {
	UDPRequestWriter
	UDPResponseReader
}

type UDP_Putter_Generator interface {
	GetNewUDP_Putter() UDP_Putter
}

//////////////////// 具体实现 ////////////////////

// 最简单的 UDP_Putter，用于客户端; 不处理内部数据，直接认为要 发送给服务端的信息 要发送到一个特定的地址
//	如果指定的地址不是 默认的地址，则发送到 unknownRemoteAddrMsgWriter
//
// 对于 vless v1来说, unknownRemoteAddrMsgWriter 要做的 就是 新建一个与服务端的 请求udp的连接，
//  然后这个新连接就变成了新的 UniUDP_Putter
type UniUDP_Putter struct {
	targetAddr net.UDPAddr
	io.ReadWriter

	unknownRemoteAddrMsgWriter UDPRequestWriter
}

//
func (e *UniUDP_Putter) GetNewUDPResponse() (net.UDPAddr, []byte, error) {
	bs := make([]byte, MaxUDP_packetLen)
	n, err := e.ReadWriter.Read(bs)
	if err != nil {
		return e.targetAddr, nil, err
	}
	return e.targetAddr, bs[:n], nil
}

func (e *UniUDP_Putter) WriteUDPRequest(addr net.UDPAddr, bs []byte, dialFunc func(targetAddr Addr) (io.ReadWriter, error)) (err error) {

	if addr.String() == e.targetAddr.String() {
		_, err = e.ReadWriter.Write(bs)

		return
	} else {
		if e.unknownRemoteAddrMsgWriter == nil {
			return
		}
		// 普通的 WriteUDPRequest需要调用 dialFunc来拨号新链接，而我们这里 直接就传递给 unknownRemoteAddrMsgWriter 了

		return e.unknownRemoteAddrMsgWriter.WriteUDPRequest(addr, bs, dialFunc)
	}

}

// 最简单的 UDP_Extractor，用于服务端; 不处理内部数据，直接认为客户端传来的内部数据的目标为一个特定值。
//	收到的响应数据的来源 如果和 targetAddr 相同的话，直接写入传入的 ReadWriter
//  收到的外界数据的来源 如果和 targetAddr 不同的话，那肯定就是使用了fullcone，那么要传入 unknownRemoteAddrMsgWriter； 如果New时传入unknownRemoteAddrMsgWriter的 是nil的话，那么意思就是不支持fullcone，将直接舍弃这一部分数据。
type UniUDP_Extractor struct {
	targetAddr net.UDPAddr
	io.ReadWriter

	unknownRemoteAddrMsgWriter UDPResponseWriter
}

// 新建，unknownRemoteAddrMsgWriter 用于写入 未知来源响应，rw 用于普通的客户请求的目标的响应
func NewUniUDP_Extractor(addr net.UDPAddr, rw io.ReadWriter, unknownRemoteAddrMsgWriter UDPResponseWriter) *UniUDP_Extractor {
	return &UniUDP_Extractor{
		targetAddr:                 addr,
		ReadWriter:                 rw,
		unknownRemoteAddrMsgWriter: unknownRemoteAddrMsgWriter,
	}
}

// 从客户端连接中 提取出 它的 UDP请求，就是直接读取数据。然后搭配上之前设置好的地址
func (e *UniUDP_Extractor) GetNewUDPRequest() (net.UDPAddr, []byte, error) {
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
func (e *UniUDP_Extractor) WriteUDPResponse(addr net.UDPAddr, bs []byte) (err error) {

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
	Addr net.UDPAddr
	Data []byte
}

//一种简单的本地 UDP_Extractor + UDP_Putter
type UDP_Pipe struct {
	requestChan, responseChan             chan UDPAddrData
	requestChanClosed, responseChanClosed bool
}

func (u *UDP_Pipe) IsInvalid() bool {
	return u.requestChanClosed || u.responseChanClosed
}

func (u *UDP_Pipe) closeRequestChan() {
	if !u.requestChanClosed {
		close(u.requestChan)
		u.requestChanClosed = true
	}
}
func (u *UDP_Pipe) closeResponseChan() {
	if !u.responseChanClosed {
		close(u.responseChan)
		u.responseChanClosed = true
	}
}

func (u *UDP_Pipe) Close() {
	u.closeRequestChan()
	u.closeResponseChan()

}

func NewUDP_Pipe() *UDP_Pipe {
	return &UDP_Pipe{
		requestChan:  make(chan UDPAddrData, 10),
		responseChan: make(chan UDPAddrData, 10),
	}
}

func (u *UDP_Pipe) CloseUDPRequestWriter() {
	u.closeRequestChan()
}

func (u *UDP_Pipe) GetNewUDPRequest() (net.UDPAddr, []byte, error) {

	d, ok := <-u.requestChan
	if ok {
		return d.Addr, d.Data, nil

	} else {
		//如果requestChan被关闭了，就要同时关闭 responseChan
		u.closeResponseChan()
		return net.UDPAddr{}, nil, io.EOF
	}
}

func (u *UDP_Pipe) GetNewUDPResponse() (net.UDPAddr, []byte, error) {
	d, ok := <-u.responseChan
	if ok {
		return d.Addr, d.Data, nil

	} else {
		//如果 responseChan 被关闭了，就要同时关闭 requestChan
		u.closeRequestChan()
		return net.UDPAddr{}, nil, io.EOF
	}

}

// 会保存bs的副本，不必担心数据被改变的问题。
func (u *UDP_Pipe) WriteUDPResponse(addr net.UDPAddr, bs []byte) error {
	bsCopy := make([]byte, len(bs))
	copy(bsCopy, bs)

	u.responseChan <- UDPAddrData{
		Addr: addr,
		Data: bsCopy,
	}
	return nil
}

// 会保存bs的副本，不必担心数据被改变的问题。
func (u *UDP_Pipe) WriteUDPRequest(addr net.UDPAddr, bs []byte, dialFunc func(targetAddr Addr) (io.ReadWriter, error)) error {
	bsCopy := make([]byte, len(bs))
	copy(bsCopy, bs)

	u.requestChan <- UDPAddrData{
		Addr: addr,
		Data: bsCopy,
	}
	return nil
}

// RelayUDP_to_Direct 用于 从一个未知协议读取 udp请求，然后通过 直接的udp连接 发送到 远程udp 地址。
// 该函数是阻塞的。而且实现了fullcone; 本函数会直接处理 对外新udp 的dial
//
// RelayUDP_to_Direct 与 RelayTCP 函数 的区别是，已经建立的udpConn是可以向其它目的地址发送信息的
// 服务端可以向 客户端发送 非客户端发送过数据 的地址 发来的信息
// 原理是，客户端请求第一次后，就会在服务端开放一个端口，然后其它远程主机就会发现这个端口并试图向客户端发送数据
//	而由于fullcone，所以如果客户端要请求一个 不同的udp地址的话，如果这个udp地址是之前发送来过信息，那么就要用之前建立过的udp连接，这样才能保证端口一致；
//
func RelayUDP_to_Direct(extractor UDP_Extractor) {

	//具体实现： 每当有对新远程udp地址的请求发生时，就会同时 监听 “用于发送该请求到远程udp主机的本地udp端口”，接受一切发往 该端口的数据

	var dialedUDPConnMap map[string]*net.UDPConn = make(map[string]*net.UDPConn)

	var mutex sync.RWMutex

	for {

		addr, requestData, err := extractor.GetNewUDPRequest()
		if err != nil {
			break
		}

		addrStr := addr.String()

		mutex.RLock()
		oldConn := dialedUDPConnMap[addrStr]
		mutex.RUnlock()

		if oldConn != nil {

			oldConn.Write(requestData)

		} else {

			newConn, err := net.DialUDP("udp", nil, &addr)
			if err != nil {

				break
			}

			_, err = newConn.Write(requestData)
			if err != nil {
				break
			}

			mutex.Lock()
			dialedUDPConnMap[addrStr] = newConn
			mutex.Unlock()

			newConn.SetDeadline(time.Now().Add(UDP_timeout))

			//监听所有发往 newConn的 远程任意主机 发来的消息。
			go func(thisconn *net.UDPConn, supposedRemoteAddr net.UDPAddr) {
				bs := make([]byte, MaxUDP_packetLen)
				for {
					//log.Println("redirect udp, start read", supposedRemoteAddr)
					n, raddr, err := thisconn.ReadFromUDP(bs)
					if err != nil {

						break
					}

					// 这个远程 地址 无论是新的还是旧的， 都是要 和 newConn关联的，下一次向 这个远程地址发消息时，也要用 newConn来发，而不是新dial一个。

					// 因为判断本身也要占一个语句，所以就不管新旧了，直接赋值即可。
					// 所以也就不需要 比对 supposedRemoteAddr 和 raddr了

					mutex.Lock()
					dialedUDPConnMap[raddr.String()] = thisconn
					mutex.Unlock()

					//log.Println("redirect udp, will write to extractor", string(bs[:n]))

					err = extractor.WriteUDPResponse(*raddr, bs[:n])
					if err != nil {
						break
					}

				}
			}(newConn, addr)
		}

	}

}
