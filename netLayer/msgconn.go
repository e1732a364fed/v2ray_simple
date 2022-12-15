package netLayer

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

// MsgConn一般用于 udp. 是一种类似 net.PacketConn 的包装.
// 实现 MsgConn接口 的类型 可以被用于 RelayUDP 进行转发。
//
// ReadMsgFrom直接返回数据, 这样可以尽量避免多次数据拷贝。
//
// 使用Addr，是因为有可能请求地址是个域名，而不是ip; 而且通过Addr, MsgConn实际上有可能支持通用的情况,
// 即可以用于 客户端 一会 请求tcp，一会又请求udp，一会又请求什么其它网络层 这种奇葩情况.
type MsgConn interface {
	NetDeadliner

	ReadMsgFrom() ([]byte, Addr, error)
	WriteMsgTo([]byte, Addr) error
	CloseConnWithRaddr(raddr Addr) error //关闭特定连接
	Close() error                        //关闭所有连接
	Fullcone() bool                      //若Fullcone, 则在转发因另一端关闭而结束后, RelayUDP函数不会Close它.
}

// 将MsgConn适配为Net.Conn
type MsgConnNetAdapter struct {
	MsgConn
	LA, RA net.Addr
}

func (ma MsgConnNetAdapter) Read(p []byte) (int, error) {
	bs, _, err := ma.MsgConn.ReadMsgFrom()
	return copy(p, bs), err
}

func (ma MsgConnNetAdapter) Write(p []byte) (int, error) {

	ra, _ := NewAddrFromAny(ma.RA)
	err := ma.MsgConn.WriteMsgTo(p, ra)
	return len(p), err
}
func (ma MsgConnNetAdapter) LocalAddr() net.Addr {
	return ma.LA
}
func (ma MsgConnNetAdapter) RemoteAddr() net.Addr {
	return ma.RA
}

// symmetric, proxy/dokodemo 有用到. 实现 MsgConn 和 net.Conn
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

// UDPMsgConn 实现 MsgConn 和 net.PacketConn。 可满足fullcone/symmetric. 在proxy/direct 被用到.
type UDPMsgConn struct {
	*net.UDPConn
	IsServer, fullcone, closed bool

	symmetricMap      map[HashableAddr]*net.UDPConn
	symmetricMapMutex sync.RWMutex

	symmetricMsgReadChan chan AddrData
}

// NewUDPMsgConn 创建一个 UDPMsgConn 并使用传入的 laddr 监听udp; 若未给出laddr, 将使用一个随机可用的端口监听.
// 如果是普通的单目标的客户端，用 (nil,false,false) 即可.
//
// 满足fullcone/symmetric, 由 fullcone 的值决定.
func NewUDPMsgConn(laddr *net.UDPAddr, fullcone bool, isserver bool, sockopt *Sockopt) (*UDPMsgConn, error) {
	uc := new(UDPMsgConn)
	var udpConn *net.UDPConn
	var err error
	if sockopt != nil {
		if laddr == nil {
			laddr = &net.UDPAddr{}
		}
		a := NewAddrFromUDPAddr(laddr)
		pConn, e := a.ListenUDP_withOpt(sockopt)
		if e != nil {
			err = e
		} else {
			udpConn = pConn.(*net.UDPConn)
		}
	} else {
		udpConn, err = net.ListenUDP("udp", laddr)

	}

	if err != nil {
		return nil, err
	}
	udpConn.SetReadBuffer(MaxUDP_packetLen)
	udpConn.SetWriteBuffer(MaxUDP_packetLen)

	uc.UDPConn = udpConn
	uc.fullcone = fullcone
	uc.IsServer = isserver
	if !fullcone {
		uc.symmetricMap = make(map[HashableAddr]*net.UDPConn)
		uc.symmetricMsgReadChan = make(chan AddrData, 50) //缓存大一点比较好. 假设有十个udp连接, 每一个都连续读了5个信息，这样就会装满50个缓存了。

		//我们暂时不把udpConn放入 symmetricMap 中，而是等待第一次Write成功后再放入.
	}
	return uc, nil
}

func (u *UDPMsgConn) Fullcone() bool {
	return u.fullcone
}

func (u *UDPMsgConn) readSymmetricMsgFromConn(conn *net.UDPConn, thishash HashableAddr) {
	if ce := utils.CanLogDebug("readSymmetricMsgFromConn called"); ce != nil {
		ce.Write(zap.String("addr", thishash.String()))
	}
	for {
		bs := utils.GetPacket()

		conn.SetReadDeadline(time.Now().Add(UDP_timeout))

		n, ad, err := conn.ReadFromUDP(bs)

		if err != nil || u.closed {
			break
		}

		u.symmetricMsgReadChan <- AddrData{Data: bs[:n], Addr: NewAddrFromUDPAddr(ad)}
	}

	u.symmetricMapMutex.Lock()
	delete(u.symmetricMap, thishash)
	u.symmetricMapMutex.Unlock()

	conn.Close()

}

func (u *UDPMsgConn) ReadMsgFrom() ([]byte, Addr, error) {
	if u.fullcone {
		bs := utils.GetPacket()

		u.UDPConn.SetReadDeadline(time.Now().Add(UDP_fullcone_timeout))

		n, ad, err := u.UDPConn.ReadFromUDP(bs)

		if err != nil {
			return nil, Addr{}, err
		}

		return bs[:n], NewAddrFromUDPAddr(ad), nil
	} else {
		ad, ok := <-u.symmetricMsgReadChan
		if ok {
			ad.Addr.Network = "udp"
			return ad.Data, ad.Addr, nil
		} else {
			return nil, Addr{}, net.ErrClosed
		}
	}

}

func (u *UDPMsgConn) WriteMsgTo(bs []byte, raddr Addr) error {

	var theConn *net.UDPConn

	if !u.fullcone && !u.IsServer {
		//非fullcone时,  强制 symmetric, 对每个远程地址 都使用一个 对应的新laddr

		//UDPMsgConn 一般用于 direct，此时 一定有 !u.IsServer 成立

		thishash := raddr.GetHashable()
		thishash.Network = "udp" //有可能调用者忘配置Network项.

		if len(u.symmetricMap) == 0 {

			_, err := u.UDPConn.WriteTo(bs, raddr.ToUDPAddr())
			if err == nil {
				u.symmetricMapMutex.Lock()
				u.symmetricMap[thishash] = u.UDPConn
				u.symmetricMapMutex.Unlock()
			}
			go u.readSymmetricMsgFromConn(u.UDPConn, thishash)
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

			go u.readSymmetricMsgFromConn(theConn, thishash)
		}

	} else {
		theConn = u.UDPConn
	}

	_, err := theConn.WriteTo(bs, raddr.ToUDPAddr())
	return err
}

func (u *UDPMsgConn) CloseConnWithRaddr(raddr Addr) error {
	if !u.IsServer {
		if u.fullcone {
			//u.UDPConn.SetReadDeadline(time.Now())

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

	if !u.closed {
		u.closed = true

		if !u.fullcone {
			close(u.symmetricMsgReadChan)
		}
		return u.UDPConn.Close()
	}

	return nil
}

// 实现 net.PacketConn
func (uc *UDPMsgConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	var bs []byte
	var a Addr
	bs, a, err = uc.ReadMsgFrom()
	if err == nil {
		n = copy(p, bs)
		addr = a.ToUDPAddr()
	}
	return
}

// 实现 net.PacketConn
func (uc *UDPMsgConn) WriteTo(p []byte, raddr net.Addr) (n int, err error) {
	if ua, ok := raddr.(*net.UDPAddr); ok {
		err = uc.WriteMsgTo(p, NewAddrFromUDPAddr(ua))
		if err == nil {
			n = len(p)
		}

	} else {
		err = errors.New("UDPMsgConn.WriteTo, raddr can't cast to *net.UDPAddr")
	}
	return

}

// Wraps net.PacketConn and implements MsgConn
type MsgConnForPacketConn struct {
	net.PacketConn
}

func (mc *MsgConnForPacketConn) ReadMsgFrom() ([]byte, Addr, error) {
	bs := utils.GetPacket()
	n, addr, err := mc.ReadFrom(bs)
	if err != nil {
		return nil, Addr{}, err
	}
	a, err := NewAddrFromAny(addr)
	if err != nil {
		return nil, Addr{}, err
	}
	return bs[:n], a, nil
}

func (mc *MsgConnForPacketConn) WriteMsgTo(p []byte, a Addr) error {
	_, err := mc.WriteTo(p, a.ToAddr())
	return err
}
func (mc *MsgConnForPacketConn) CloseConnWithRaddr(raddr Addr) error {
	return mc.PacketConn.Close()

}
func (mc *MsgConnForPacketConn) Close() error {
	return mc.PacketConn.Close()
}
func (mc *MsgConnForPacketConn) Fullcone() bool {
	return true
}

// Wraps net.PacketConn and implements MsgConn
type UniSourceMsgConnForPacketConn struct {
	net.PacketConn
	Source Addr
}

func (mc *UniSourceMsgConnForPacketConn) ReadMsgFrom() ([]byte, Addr, error) {
	bs := utils.GetPacket()
	n, _, err := mc.ReadFrom(bs)
	if err != nil {
		return nil, mc.Source, err
	}

	return bs[:n], mc.Source, nil
}

func (mc *UniSourceMsgConnForPacketConn) WriteMsgTo(p []byte, a Addr) error {
	_, err := mc.WriteTo(p, a.ToAddr())
	return err
}
func (mc *UniSourceMsgConnForPacketConn) CloseConnWithRaddr(raddr Addr) error {
	return mc.PacketConn.Close()

}
func (mc *UniSourceMsgConnForPacketConn) Close() error {
	return mc.PacketConn.Close()
}
func (mc *UniSourceMsgConnForPacketConn) Fullcone() bool {
	return true
}
