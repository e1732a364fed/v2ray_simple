package netLayer

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
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
// 实现 MsgConn接口 的类型 可以被用于 RelayUDP 进行转发。
//
//ReadMsgFrom直接返回数据, 这样可以尽量避免多次数据拷贝。
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

/*
udp是无连接的，所以需要考虑超时问题。
在转发udp时, 有可能有多种情况：

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

		所以依然需要在服务端 的 direct上面 加Read 时限,否则 rc.ReadFrom() 会卡住而不返回.

		direct 使用的 UDPMsgConn 会自动设置超时，所以可以放心。

	4. fullcone, 此时不能对整个监听端口进行close，会影响其它外部链接发来的连接。

	5. vless v1 这种单路client的udp转发方式, 此时需要判断lc.ReadMsgFrom得到的 raddr是否是已知地址,
		如果是未知的, 则不会再使用原来的rc，而是要拨号新通道

		也就是说，lc是有且仅有一个的, 因为是socks5 / dokodemo都是采用的单信道的方式,

		而在 vless v1时, udp的rc的拨号可以采用多信道方式。

	socks5作为lc 是fullcone的，而 dokodemo 作为 lc 是 symmetric的

	trojan 作为 lc/rc 都是 fullcone的。 direct 作为 rc, 是fullcone还是symmetric是可调节的。

	fullcone 的 lc一般是很难产生错误的, 因为它没有时限; 多路复用的rc也因为不能设时限, 所以也很难产生错误.

	所以 转发循环 的退出 一般是在 fullcone 的 lc 从 symmetric 的 rc 读取时超时 时 产生的。

	而分离信道法中，因为每一个rc都是独立的连接, 所以就算是fullcone, 似乎也可以设置 rc读取超时

	但是, vless v1虽然是分离信道, 但还有可能读到 umfurs信息, 所以还是不应设置超时.
*/

// 阻塞. 返回从 rc 下载的总字节数. 拷贝完成后自动关闭双端连接.
func RelayUDP(rc, lc MsgConn, downloadByteCount, uploadByteCount *uint64) uint64 {
	go func() {
		var count uint64

		for {
			bs, raddr, err := lc.ReadMsgFrom()
			if err != nil {
				break
			}

			if ce := utils.CanLogDebug("RelayUDP will write to"); ce != nil {
				ce.Write(zap.String("raddr", raddr.String()), zap.Int("len", len(bs)))
			}

			err = rc.WriteMsgTo(bs, raddr)
			if err != nil {

				break
			}

			count += uint64(len(bs))
		}
		if !rc.Fullcone() {
			rc.Close()
		}

		if !lc.Fullcone() {
			lc.Close()
		}

		if uploadByteCount != nil {
			atomic.AddUint64(uploadByteCount, count)
		}

	}()

	count2, _ := relayUDP_rc_toLC(rc, lc, downloadByteCount, nil)
	return count2
}

//循环从rc读取数据，并写入lc，直到错误发生。若 downloadByteCount 给出，会更新 下载总字节数。
// 返回此次所下载的字节数。如果是rc读取产生了错误导致的退出, 返回的bool为true
func relayUDP_rc_toLC(rc, lc MsgConn, downloadByteCount *uint64, mutex *sync.RWMutex) (uint64, bool) {
	//utils.Debug("relayUDP_rc_toLC called")
	var count uint64
	var rcwrong bool
	for {
		bs, raddr, err := rc.ReadMsgFrom()
		if err != nil {
			rcwrong = true
			break
		}

		//if ce := utils.CanLogDebug("relayUDP_rc_toLC got msg from rc"); ce != nil {
		//	ce.Write(zap.String("raddr", raddr.String()), zap.Int("len", len(bs)))
		//}
		if mutex != nil {
			mutex.Lock()
			err = lc.WriteMsgTo(bs, raddr)
			mutex.Unlock()

		} else {
			err = lc.WriteMsgTo(bs, raddr)

		}
		if err != nil {
			break
		}
		count += uint64(len(bs))
	}
	if !rc.Fullcone() {
		rc.Close()
	}

	if !lc.Fullcone() {
		lc.Close()
	}

	if downloadByteCount != nil {
		atomic.AddUint64(downloadByteCount, count)
	}

	return count, rcwrong
}

// RelayUDP_separate 对 lc 读到的每一个新raddr地址 都新拨号一个rc. 这样就避开了经典的udp多路复用转发的效率低下问题.
// separate含义就是 【分离信道】。随着时间推移, 会创建多个rc。
// 分离信道法还有个好处，就是fullcone时，不必一直保留某连接, 如果超时/读取错误, 可以断开单个rc连接, 释放占用的端口资源.
// 不过分离信道只能用于代理，不能用于 direct, 因为direct为了实现fullcone, 对所有rc连接都用的同一个udp端口。
// 阻塞. 返回从 rc 下载的总字节数. 拷贝完成后自动关闭双端连接.
func RelayUDP_separate(rc, lc MsgConn, firstAddr *Addr, downloadByteCount, uploadByteCount *uint64, dialfunc func(raddr Addr) MsgConn) uint64 {
	var lc_mutex sync.RWMutex

	utils.Debug("RelayUDP_separate called")

	go func() {
		var count uint64
		//从单个lc读取, 然后随着时间推移, 会创建多个rc.
		// 然后 对每一个rc, 创建单独goroutine 读取rc, 然后写入lc.
		// 因为是多通道的, 所以涉及到了 对 lc 写入的 并发抢占问题, 要加锁。

		rc_raddrMap := make(map[HashableAddr]MsgConn)
		if firstAddr != nil {
			rc_raddrMap[firstAddr.GetHashable()] = rc
		}

		for {
			bs, raddr, err := lc.ReadMsgFrom()
			if err != nil {
				break
			}
			hash := raddr.GetHashable()

			lc_mutex.RLock()
			oldrc := rc_raddrMap[hash]
			lc_mutex.RUnlock()

			if oldrc != nil {
				//utils.Debug("RelayUDP_separate got old")

				rc = oldrc
			} else {
				utils.Debug("RelayUDP_separate dial new")

				rc = dialfunc(raddr)
				if rc == nil {
					continue
				}

				lc_mutex.Lock()
				rc_raddrMap[hash] = rc
				lc_mutex.Unlock()

				go func() {
					_, rcwrong := relayUDP_rc_toLC(rc, lc, downloadByteCount, &lc_mutex)
					//rc到lc转发结束，一定也是因为读取/写入失败, 如果是rc的错误, 则我们要删掉rc, 释放资源

					if rcwrong {
						lc_mutex.Lock()
						delete(rc_raddrMap, hash)
						lc_mutex.Unlock()

						rc.Close()
					}

				}()
			}

			err = rc.WriteMsgTo(bs, raddr)
			if err != nil {

				lc_mutex.Lock()
				delete(rc_raddrMap, hash)
				lc_mutex.Unlock()

				rc.Close()

				//我们分离信道法，一个 写通道 的断开 并不意味着 所有写通道 都废掉。
				continue
			}

			count += uint64(len(bs))
		}
		//上面循环 只有lc 读取失败时才会退出, 此时因为我们不是多路复用, 所以可以放心close

		lc_mutex.Lock()
		for _, thisrc := range rc_raddrMap {
			thisrc.Close()
		}
		lc_mutex.Unlock()

		lc.Close()

		if uploadByteCount != nil {
			atomic.AddUint64(uploadByteCount, count)
		}

	}()

	count2, _ := relayUDP_rc_toLC(rc, lc, downloadByteCount, &lc_mutex)
	return count2
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

//UDPMsgConn 实现 MsgConn。 可满足fullcone/symmetric. 在proxy/direct 被用到.
type UDPMsgConn struct {
	conn                       *net.UDPConn
	IsServer, fullcone, closed bool

	symmetricMap      map[HashableAddr]*net.UDPConn
	symmetricMapMutex sync.RWMutex

	symmetricMsgReadChan chan AddrData
}

// NewUDPMsgConn 创建一个 UDPMsgConn 并使用传入的 laddr 监听udp; 若未给出laddr, 将使用一个随机可用的端口监听.
// 如果是普通的单目标的客户端，用 (nil,false,false) 即可.
//
// 满足fullcone/symmetric, 由 fullcone 的值决定.
func NewUDPMsgConn(laddr *net.UDPAddr, fullcone bool, isserver bool) (*UDPMsgConn, error) {
	uc := new(UDPMsgConn)

	udpConn, err := net.ListenUDP("udp", laddr) //根据反映，这里是有可能报错的，以后可以考虑重试多次。
	if err != nil {
		return nil, err
	}
	udpConn.SetReadBuffer(MaxUDP_packetLen)
	udpConn.SetWriteBuffer(MaxUDP_packetLen)

	uc.conn = udpConn
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

		if err != nil {
			break
		}
		//conn.SetReadDeadline(time.Time{})

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

		n, ad, err := u.conn.ReadFromUDP(bs)

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

		thishash := raddr.GetHashable()
		thishash.Network = "udp" //有可能调用者忘配置Network项.

		if len(u.symmetricMap) == 0 {

			_, err := u.conn.WriteTo(bs, raddr.ToUDPAddr())
			if err == nil {
				u.symmetricMapMutex.Lock()
				u.symmetricMap[thishash] = u.conn
				u.symmetricMapMutex.Unlock()
			}
			go u.readSymmetricMsgFromConn(u.conn, thishash)
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

	if !u.closed {
		u.closed = true

		if !u.fullcone {
			close(u.symmetricMsgReadChan)
		}
		return u.conn.Close()
	}

	return nil
}
