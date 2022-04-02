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

//UDPConn 实现了 net.Conn
type UDPConn struct {
	peerAddr *net.UDPAddr
	realConn *net.UDPConn

	inMsgChan chan []byte

	readDeadline  PipeDeadline
	writeDeadline PipeDeadline

	clientFirstWriteChan       chan int
	clientFirstWriteChanClosed bool

	unread   []byte
	isClient bool
}

func DialUDP(raddr *net.UDPAddr) (net.Conn, error) {
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, err
	}
	return NewUDPConn(raddr, conn, true), nil
}

//如果isClient为true，则本函数返回后，必须要调用一次 Write，才能在Read读到数据
func NewUDPConn(raddr *net.UDPAddr, conn *net.UDPConn, isClient bool) *UDPConn {
	inDataChan := make(chan []byte, 50)
	theUDPConn := &UDPConn{raddr, conn, inDataChan, MakePipeDeadline(),
		MakePipeDeadline(), make(chan int), false, []byte{}, isClient}

	//不设置缓存的话，会导致发送过快 而导致丢包
	conn.SetReadBuffer(MaxUDP_packetLen)
	conn.SetWriteBuffer(MaxUDP_packetLen)

	if isClient {

		//客户端要自己循环读取udp,(但是要等待客户端自己先Write之后)
		go func() {
			<-theUDPConn.clientFirstWriteChan
			for {
				buf := utils.GetPacket()
				n, _, err := conn.ReadFromUDP(buf)

				//这里默认认为每个客户端都是在NAT后的,不怕遇到其它raddr,
				// 即默认认为只可能读到 我们服务器发来的数据.

				inDataChan <- buf[:n] //该数据会被ReadMsg和 Read读到

				if err != nil {
					break
				}
			}
		}()

	}
	return theUDPConn
}

func (uc *UDPConn) ReadMsg() (b []byte, err error) {

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

//实现 net.PacketConn， 可以与 miekg/dns 配合
func (uc *UDPConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
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

//实现 net.PacketConn， 可以与 miekg/dns 配合
func (uc *UDPConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return uc.Write(p)

}

func (uc *UDPConn) GetReadChan() chan []byte {
	return uc.inMsgChan
}

func (uc *UDPConn) Read(buf []byte) (n int, err error) {
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

func (uc *UDPConn) Write(buf []byte) (n int, err error) {
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

func (uc *UDPConn) Close() error {
	if uc.isClient {
		return uc.realConn.Close()
	}
	return nil
}

func (b *UDPConn) LocalAddr() net.Addr         { return b.realConn.LocalAddr() }
func (b *UDPConn) RemoteAddr() net.Addr        { return b.peerAddr }
func (b *UDPConn) RemoteUDPAddr() *net.UDPAddr { return b.peerAddr }

func (b *UDPConn) SetWriteDeadline(t time.Time) error {
	b.writeDeadline.Set(t)
	return nil
}
func (b *UDPConn) SetReadDeadline(t time.Time) error {
	b.readDeadline.Set(t)
	return nil
}
func (b *UDPConn) SetDeadline(t time.Time) error {
	b.readDeadline.Set(t)
	b.writeDeadline.Set(t)
	return nil
}
