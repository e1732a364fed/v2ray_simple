package netLayer

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

//本文件内含 转发 udp 数据的方法

const (
	MaxUDP_packetLen = 64 * 1024 // 关于 udp包数据长度，可参考 https://cloud.tencent.com/developer/article/1021196

)

var (
	//udp不能无限监听, 否则每一个udp申请都对应打开了一个本地udp端口，一直监听的话时间一长，就会导致 too many open files
	// 因为实际上udp在网页代理中主要用于dns请求, 所以不妨设的小一点。
	// 放心，只要能持续不断地从远程服务器收到数据, 建立的udp连接就会持续地更新Deadline 而续命一段时间.
	UDP_timeout = time.Minute * 3

	/*
		fullcone时，wlc 监听本地随机udp端口，而且时刻准备接收 其它端口发来的信息，所以 某个 wrc 被关闭后，wlc 不能随意被关闭；相反，如果 wlc的读取 或写入 遇到 错误而推出后，可以关闭 wrc 和 wlc。

		又因为 vless v1 普通模式 和 trojan 是 在多路复用的 一个通道上 传输 udp的，所以 就算 wrc 发生错误，wlc依然不应该被关闭，否则会影响到 其它流量的传输；

		还有就是，因为 udp 是 无状态的，所以 基本上很难遇到 udp读取失败的情况，一般都是会一直卡住，所以确实需要我们设置超时
	*/
	UDP_fullcone_timeout = time.Minute * 30
)

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

/*
	阻塞. 返回从 rc 下载的总字节数. 拷贝完成后，如不为fullcone，则自动关闭双端连接.

若为fullcone，则 rc错误时，rc可以关闭，而 lc 则不可以随意关闭; 若lc错误时，则两端都可关闭
*/
func RelayUDP(rc, lc MsgConn, downloadByteCount, uploadByteCount *uint64) uint64 {
	isfullcone := rc.Fullcone() && lc.Fullcone()
	go func() {
		var count uint64

		var lcReadErr bool

		for {
			bs, raddr, err := lc.ReadMsgFrom()
			if err != nil {
				lcReadErr = true
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

		if !isfullcone {
			rc.Close()
			lc.Close()
		} else {

			rc.Close()

			if lcReadErr {
				lc.Close()
			}

		}

		if uploadByteCount != nil {
			atomic.AddUint64(uploadByteCount, count)
		}

	}()

	count2, rcReadErr := relayUDP_rc_toLC(rc, lc, downloadByteCount, nil)
	rc.Close()

	if isfullcone {

		if !rcReadErr {
			lc.Close()
		}
	} else {
		lc.Close()
	}

	return count2
}

/*
循环从rc读取数据，并写入lc，直到错误发生。若 downloadByteCount 给出，会更新 下载总字节数。
返回此次所下载的字节数。如果是rc读取产生了错误导致的退出, 返回的bool为true。若mutex给出，则 内部调用 lc.WriteMsgTo 时会进行 锁定。
*/
func relayUDP_rc_toLC(rc, lc MsgConn, downloadByteCount *uint64, mutex *sync.RWMutex) (uint64, bool) {

	var count uint64
	var rcwrong bool
	for {
		bs, raddr, err := rc.ReadMsgFrom()
		if err != nil {
			rcwrong = true
			break
		}

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

	var mainhash HashableAddr

	utils.Debug("RelayUDP_separate called")

	rc_raddrMap := make(map[HashableAddr]MsgConn)
	if firstAddr != nil {

		mainhash = firstAddr.GetHashable()

		rc_raddrMap[mainhash] = rc
	}

	go func() {
		var count uint64
		//从单个lc读取, 然后随着时间推移, 会创建多个rc.
		// 然后 对每一个rc, 创建单独goroutine 读取rc, 然后写入lc.
		// 因为是多通道的, 所以涉及到了 对 lc 写入的 并发抢占问题, 要加锁。

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
					//一般而言，lc为 socks5 的MsgConn，rc 为 vless v1 的 MsgConn

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
		//上面循环 只有lc 读取失败时才会退出,

		//lc退出后，我们要 关闭所有rc连接。此时因为我们不是多路复用, 所以可以放心close

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

	count2, rcwrong := relayUDP_rc_toLC(rc, lc, downloadByteCount, &lc_mutex)
	if rcwrong {
		lc_mutex.Lock()
		delete(rc_raddrMap, mainhash)
		lc_mutex.Unlock()

		rc.Close()

	}
	return count2
}

/*
relay udp  有两种针对不同通道的技术

一种是针对单来源通道的技术，通常是udp in tcp的情况，此时，我们用MsgConn + RelayUDP 方法很方便

另一种是直接读udp的技术，目前我们用很多代码来适配它，也能包装MsgConn里，但是非常不美观，繁琐。
因为udp是多来源的，我们为了确定单一来源，就要全局读，然后定义一个map, 为每一个来源的地址存储一个MsgConn

我们重新定义一个MsgProducer 和 MsgConsumer，就方便很多. 这是针对多来源的转发。

如此，一个 UDPConn就相当于一个 MsgProducer, 它的to 可以由 tproxy 或者 msg内部的数据提取出来

而且 这个模型也可以实现 单来源，所以更实用
*/
func RelayMsg(rc, lc MsgHub, downloadByteCount, uploadByteCount *uint64) uint64 {
	go CopyMsgFromP2C(lc, rc, uploadByteCount)

	var dbc uint64
	CopyMsgFromP2C(rc, lc, &dbc)

	if downloadByteCount != nil {
		atomic.AddUint64(downloadByteCount, dbc)
	}
	return dbc

}

func CopyMsgFromP2C(p MsgProducer, c MsgConsumer, countPtr *uint64) {
	var bc uint64

	for {
		msg, from, to, err := p.ProduceMsg()
		if err != nil {
			break
		}
		err = c.ConsumeMsg(msg, from, to)
		if err != nil {
			break
		}
		bc += uint64(len(msg))
	}

	if countPtr != nil {
		atomic.AddUint64(countPtr, bc)
	}

}

type MsgHub interface {
	MsgProducer
	MsgConsumer
}

type MsgProducer interface {
	ProduceMsg() (msg []byte, from, to Addr, err error)
}

type MsgConsumer interface {
	ConsumeMsg(msg []byte, from, to Addr) (err error)
}
