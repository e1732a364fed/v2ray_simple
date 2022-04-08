package netLayer

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

var (
	ErrTimeout = errors.New("timeout")
)

//Uni_UDPConn将一个udp连接包装成一个 只能向单一目标发送数据的 连接。
//Uni_UDPConn 实现了 net.Conn 和 net.PacketConn
type Uni_UDPConn struct {
	peerAddr *net.UDPAddr
	realConn *net.UDPConn

	inMsgChan       chan []byte
	inMsgChanClosed bool

	readDeadline  PipeDeadline
	writeDeadline PipeDeadline

	clientFirstWriteChan       chan int
	clientFirstWriteChanClosed bool

	unread   []byte
	isClient bool
}

//我们这里为了保证udp连接不会一直滞留导致 too many open files的情况,
// 主动设置了 内层udp连接的 read的 timeout为 UDP_timeout。
// 你依然可以设置 DialUDP 所返回的 net.Conn 的 Deadline, 这属于外层的Deadline,
// 不会影响底层 udp所强制设置的 deadline.
func DialUDP(raddr *net.UDPAddr) (*Uni_UDPConn, error) {
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, err
	}
	return NewUDPConn(raddr, conn, true), nil
}

//如果isClient为true，则本函数返回后，必须要调用一次 Write，才能在Read读到数据. 这是udp的原理所决定的。
// 在客户端没有Write之前，该udp连接实际上根本没有被建立, Read也就不可能/不应该 读到任何东西.
func NewUDPConn(raddr *net.UDPAddr, conn *net.UDPConn, isClient bool) *Uni_UDPConn {
	inDataChan := make(chan []byte, 20)
	theUDPConn := &Uni_UDPConn{raddr, conn, inDataChan, false, MakePipeDeadline(),
		MakePipeDeadline(), make(chan int), false, []byte{}, isClient}

	//不设置缓存的话，会导致发送过快 而导致丢包
	conn.SetReadBuffer(MaxUDP_packetLen)
	conn.SetWriteBuffer(MaxUDP_packetLen)

	if isClient {

		//客户端要自己循环读取udp,(但是要等待客户端自己先Write之后)
		//我们这里为了保证udp连接不会一直滞留导致 too many open files的情况,
		// 主动设置了 timeout为 UDP_timeout

		go func() {
			<-theUDPConn.clientFirstWriteChan
			for {
				buf := utils.GetPacket()

				conn.SetReadDeadline(time.Now().Add(UDP_timeout))
				n, _, err := conn.ReadFromUDP(buf)

				//这里默认认为每个客户端都是在NAT后的,不怕遇到其它raddr,
				// 即默认认为只可能读到 我们服务器发来的数据.

				if n > 0 {
					inDataChan <- buf[:n] //该数据会被ReadMsg和 Read读到
				}

				if err != nil {
					theUDPConn.Close()
					break
				}
			}
		}()

	}
	return theUDPConn
}

func (uc *Uni_UDPConn) ReadMsg() (b []byte, err error) {

	select {
	case msg, ok := <-uc.inMsgChan:
		if !ok {
			return nil, io.EOF
		}
		return msg, nil

	case <-uc.readDeadline.Wait():
		return nil, ErrTimeout
	}
}

//实现 net.PacketConn， 可以与 miekg/dns 配合。返回的 addr 只可能为 之前预先配置的远程目标地址
func (uc *Uni_UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case msg, ok := <-uc.inMsgChan:
		if !ok {
			return 0, uc.peerAddr, io.EOF
		}
		n = copy(msg, p)
		return n, uc.peerAddr, io.EOF

	case <-uc.readDeadline.Wait():
		return 0, uc.peerAddr, ErrTimeout
	}
}

//实现 net.PacketConn， 可以与 miekg/dns 配合。会无视传入的地址, 而使用 之前预先配置的远程目标地址
func (uc *Uni_UDPConn) WriteTo(p []byte, _ net.Addr) (n int, err error) {
	return uc.Write(p)

}

func (uc *Uni_UDPConn) GetReadChan() chan []byte {
	return uc.inMsgChan
}

func (uc *Uni_UDPConn) Read(buf []byte) (n int, err error) {
	if len(uc.unread) > 0 {
		n = copy(buf, uc.unread)
		uc.unread = uc.unread[n:]
		return
	}
	var msg []byte

	msg, err = uc.ReadMsg()
	if err != nil {
		return
	}
	n = copy(buf, msg)

	diff := len(msg) - n
	if diff > 0 { //最好不要分段读，否则我们将不会把缓存放回pool，总之建议buf直接使用 utils.GetPacket

		uc.unread = append(uc.unread, msg[n:]...)
	} else {
		//我们Read时统一用的 GetPacket, 所以整个拷贝完后可以放回
		utils.PutPacket(msg)
	}

	return
}

func (uc *Uni_UDPConn) Write(buf []byte) (n int, err error) {
	select {
	case <-uc.writeDeadline.Wait():
		return 0, ErrTimeout
	default:
		if uc.isClient {
			time.Sleep(time.Millisecond) //不能发送太快，否则会出现丢包,实测简单1毫秒即可避免

			/*
				一些常见的丢包后出现的错误：

				tls
				bad mac

				ws
				non-zero rsv bits with no extension negotiated, Data: 0

				裸奔：curl客户端:
				curl: (56) LibreSSL SSL_read: error:1404C3FC:SSL routines:ST_OK:sslv3 alert bad record mac, errno 0
			*/

			//if use writeToUDP at client end, we will get err Write write udp 127.0.0.1:50361->:60006: use of WriteTo with pre-connected connection

			if !uc.clientFirstWriteChanClosed {
				defer func() {
					close(uc.clientFirstWriteChan)
					uc.clientFirstWriteChanClosed = true
				}()
			}
			return uc.realConn.Write(buf)
		} else {
			return uc.realConn.WriteToUDP(buf, uc.peerAddr)
		}

	}
}

func (uc *Uni_UDPConn) CloseMsgChan() {
	if uc.isClient {
		if !uc.clientFirstWriteChanClosed {
			uc.clientFirstWriteChanClosed = true
			close(uc.inMsgChan)
		}
	}
}

func (uc *Uni_UDPConn) Close() error {
	if uc.isClient {
		uc.CloseMsgChan()
		return uc.realConn.Close()
	}
	return nil
}

func (b *Uni_UDPConn) LocalAddr() net.Addr         { return b.realConn.LocalAddr() }
func (b *Uni_UDPConn) RemoteAddr() net.Addr        { return b.peerAddr }
func (b *Uni_UDPConn) RemoteUDPAddr() *net.UDPAddr { return b.peerAddr }

func (b *Uni_UDPConn) SetWriteDeadline(t time.Time) error {
	b.writeDeadline.Set(t)
	return nil
}
func (b *Uni_UDPConn) SetReadDeadline(t time.Time) error {
	b.readDeadline.Set(t)
	return nil
}
func (b *Uni_UDPConn) SetDeadline(t time.Time) error {
	b.readDeadline.Set(t)
	b.writeDeadline.Set(t)
	return nil
}
