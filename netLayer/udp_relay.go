package netLayer

import (
	"net"
	"sync"
	"time"

	"github.com/hahahrfool/v2ray_simple/utils"
)

const (
	MaxUDP_packetLen = 64 * 1024 // 关于 udp包数据长度，可参考 https://cloud.tencent.com/developer/article/1021196

)

var (
	//udp不能无限监听, 否则每一个udp申请都对应打开了一个本地udp端口，一直监听的话时间一长，就会导致 too many open files
	// 因为实际上udp在网页代理中主要用于dns请求, 所以不妨设的小一点。
	// 放心，只要能持续不断地从远程服务器收到数据, 建立的udp连接就会持续地更新Deadline 而续命一段时间.
	UDP_timeout = time.Minute * 3
)

//本文件内含 一些 转发 udp 数据的 接口与方法

//MsgConn一般用于 udp. 是一种类似 net.PacketConn 的包装.
// 实现 MsgConn接口 的类型 可以被用于 RelayUDP 进行转发
//
//ReadMsgFrom直接返回数据, 这样可以尽量避免多次数据拷贝
//
//使用Addr，是因为有可能请求地址是个域名，而不是ip; 而且通过Addr, MsgConn实际上有可能支持通用的情况,
// 即可以用于 客户端 一会 请求tcp，一会又请求udp，一会又请求什么其它网络层 这种奇葩情况.
type MsgConn interface {
	ReadMsgFrom() ([]byte, Addr, error)
	WriteMsgTo([]byte, Addr) error
	CloseConnWithRaddr(raddr Addr) error //关闭特定连接
	Close() error                        //关闭所有连接
	Fullcone() bool                      //若Fullcone, 则在转发因另一端关闭而结束后, RelayUDP函数不会Close它.
}

// 阻塞. 返回从 rc 下载的总字节数. 拷贝完成后自动关闭双端连接.
func RelayUDP(rc, lc MsgConn) int {

	//在转发时, 有可能有多种情况
	/*
		1. dokodemo 监听udp 定向 导向到 direct 的远程udp实际地址
			此时因为是定向的, 所以肯定不是fullcone

			dokodemo 用的是 UniTargetMsgConn, underlay 是 netLayer.UDPConn, 其已经设置了UDP_timeout

			在 netLayer.UDPConn 超时后, ReadFrom 就会解放, 并触发双向Close, 来关闭我们的 direct的udp连接。

		1.5. 比较少见的情况, dokodemo监听tcp, 然后发送到 direct 的udp. 此时客户应用程序可以手动关闭tcp连接来帮我们触发 udp连接 的 close

		2. socks5监听 udp, 导向到 direct 的远程udp实际地址

			socks5端只用一个udp连接来监听所有信息, 所以不能关闭, 所以没有设置超时

			此时我们需要对 每一个 direct的udp连接 设置超时, 否则就会一直占用端口

		3. socks5 监听udp, 导向到 trojan, 然后 服务端的 trojan 再导向 direct

			trojan 也是用一个信道来接收udp的所有请求的, 所以trojan的连接也不能关.

			所以依然需要在服务端 的 direct上面 加Read 时限

			否则 rc.ReadFrom() 会卡住而不返回.

			因为direct 使用 UDPMsgConnWrapper，而我们已经在 UDPMsgConnWrapper里加了这个逻辑, 所以可以放心了.

		4. fullcone, 此时不能对整个监听端口进行close，会影响其它外部链接发来的连接。

	*/

	go func() {
		for {
			bs, raddr, err := lc.ReadMsgFrom()
			if err != nil {
				break
			}
			err = rc.WriteMsgTo(bs, raddr)
			if err != nil {

				break
			}
		}
		if !rc.Fullcone() {
			rc.Close()
		}

		if !lc.Fullcone() {
			lc.Close()
		}

	}()

	count := 0

	for {
		bs, raddr, err := rc.ReadMsgFrom()
		if err != nil {

			break
		}
		err = lc.WriteMsgTo(bs, raddr)
		if err != nil {

			break
		}
		count += len(bs)
	}
	if !rc.Fullcone() {
		rc.Close()
	}

	if !lc.Fullcone() {
		lc.Close()
	}
	return count
}

// symmetric, proxy/dokodemo 有用到. 实现 MsgConn
type UniTargetMsgConn struct {
	net.Conn
	Target Addr
}

func (u UniTargetMsgConn) Fullcone() bool {
	return false
}

func (u UniTargetMsgConn) ReadMsgFrom() ([]byte, Addr, error) {
	bs := utils.GetPacket()

	n, err := u.Conn.Read(bs)
	if err != nil {
		return nil, Addr{}, err
	}
	return bs[:n], u.Target, err
}

func (u UniTargetMsgConn) WriteMsgTo(bs []byte, _ Addr) error {
	_, err := u.Conn.Write(bs)
	return err
}

func (u UniTargetMsgConn) CloseConnWithRaddr(raddr Addr) error {
	return u.Conn.Close()
}

func (u UniTargetMsgConn) Close() error {
	return u.Conn.Close()
}

//UDPMsgConn 实现 MsgConn。 可满足fullcone, 由 Fullcone 的值决定. 在proxy/direct 被用到.
//
type UDPMsgConn struct {
	conn     *net.UDPConn
	IsServer bool
	fullcone bool

	symmetricMap      map[HashableAddr]*net.UDPConn
	symmetricMapMutex sync.RWMutex
}

// NewUDPMsgConn 创建一个 UDPMsgConn 并使用传入的 laddr 监听udp; 若未给出laddr, 将使用一个随机可用的端口监听.
// 如果是普通的单目标的客户端，用 (nil,false,false) 即可.
func NewUDPMsgConn(laddr *net.UDPAddr, fullcone bool, isserver bool) *UDPMsgConn {
	uc := new(UDPMsgConn)

	udpConn, _ := net.ListenUDP("udp", laddr)

	uc.conn = udpConn
	uc.fullcone = fullcone
	uc.IsServer = isserver
	if !fullcone {
		uc.symmetricMap = make(map[HashableAddr]*net.UDPConn)
	}
	return uc
}

func (u *UDPMsgConn) Fullcone() bool {
	return u.fullcone
}

func (u *UDPMsgConn) ReadMsgFrom() ([]byte, Addr, error) {
	bs := utils.GetPacket()

	if !u.fullcone {
		u.conn.SetReadDeadline(time.Now().Add(UDP_timeout))
	}

	n, ad, err := u.conn.ReadFromUDP(bs)
	if err != nil {
		return nil, Addr{}, err
	}
	if !u.fullcone {
		u.conn.SetReadDeadline(time.Time{})
	}

	return bs[:n], NewAddrFromUDPAddr(ad), nil
}

func (u *UDPMsgConn) WriteMsgTo(bs []byte, raddr Addr) error {

	var theConn *net.UDPConn

	if !u.fullcone && !u.IsServer {
		//非fullcone时,  强制 symmetric, 对每个远程地址 都使用一个 对应的新laddr

		thishash := raddr.GetHashable()

		if len(u.symmetricMap) == 0 {

			_, err := u.conn.WriteTo(bs, raddr.ToUDPAddr())
			if err == nil {
				u.symmetricMapMutex.Lock()
				u.symmetricMap[thishash] = u.conn
				u.symmetricMapMutex.Unlock()
			}
			return err
		}

		u.symmetricMapMutex.RLock()
		theConn = u.symmetricMap[thishash]
		u.symmetricMapMutex.RUnlock()

		if theConn == nil {
			var e error
			theConn, e = net.ListenUDP("udp", nil)
			if e != nil {
				return e
			}

			u.symmetricMapMutex.Lock()
			u.symmetricMap[thishash] = theConn
			u.symmetricMapMutex.Unlock()
		}

	} else {
		theConn = u.conn
	}

	_, err := theConn.WriteTo(bs, raddr.ToUDPAddr())
	return err
}

func (u *UDPMsgConn) CloseConnWithRaddr(raddr Addr) error {
	if !u.IsServer {
		if u.fullcone {
			//u.conn.SetReadDeadline(time.Now())

		} else {
			u.symmetricMapMutex.Lock()

			thehash := raddr.GetHashable()
			theConn := u.symmetricMap[thehash]

			if theConn != nil {
				delete(u.symmetricMap, thehash)
				theConn.Close()

			}

			u.symmetricMapMutex.Unlock()
		}
	}
	return nil
}

func (u *UDPMsgConn) Close() error {

	return u.conn.Close()

}
