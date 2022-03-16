// Package direct provies direct proxy support for proxy.Client
package direct

import (
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

const name = "direct"

func init() {
	proxy.RegisterClient(name, NewDirectClient)
}

type Direct struct {
	proxy.ProxyCommonStruct
	*proxy.UDP_Pipe

	targetAddr *netLayer.Addr
	addrStr    string
}

func NewDirectClient(url *url.URL) (proxy.Client, error) {
	d := &Direct{
		UDP_Pipe: proxy.NewUDP_Pipe(),
	}
	go RelayUDP_to_Direct(d.UDP_Pipe)
	return d, nil
}

func (d *Direct) Name() string { return name }

func (d *Direct) Handshake(underlay net.Conn, target *netLayer.Addr) (io.ReadWriter, error) {

	if underlay == nil {
		d.targetAddr = target
		d.SetAddrStr(d.targetAddr.String())
		return target.Dial()
	}

	return underlay, nil

}

// RelayUDP_to_Direct 用于 从一个未知协议读取 udp请求，然后通过 直接的udp连接 发送到 远程udp 地址。
// 该函数是阻塞的。而且实现了fullcone; 本函数会直接处理 对外新udp 的dial
//
// RelayUDP_to_Direct 与 RelayTCP 函数 的区别是，已经建立的udpConn是可以向其它目的地址发送信息的
// 服务端可以向 客户端发送 非客户端发送过数据 的地址 发来的信息
// 原理是，客户端请求第一次后，就会在服务端开放一个端口，然后其它远程主机就会发现这个端口并试图向客户端发送数据
//	而由于fullcone，所以如果客户端要请求一个 不同的udp地址的话，如果这个udp地址是之前发送来过信息，那么就要用之前建立过的udp连接，这样才能保证端口一致；
//
func RelayUDP_to_Direct(extractor proxy.UDP_Extractor) {

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

			newConn, err := net.DialUDP("udp", nil, addr)
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

			//监听所有发往 newConn的 远程任意主机 发来的消息。
			go func(thisconn *net.UDPConn, supposedRemoteAddr *net.UDPAddr) {
				bs := make([]byte, proxy.MaxUDP_packetLen)
				for {
					n, raddr, err := thisconn.ReadFromUDP(bs)
					if err != nil {
						break
					}

					// 这个远程 地址 无论是新的还是旧的， 都是要 和 newConn关联的，下一次向 这个远程地址发消息时，也要用 newConn来发，而不是新dial一个。

					// 因为判断本身也要占一个语句，所以就不管新旧了，直接赋值即可。
					// 所以也就不需要 比对 supposedRemoteAddr 和 raddr了

					mutex.Lock()
					dialedUDPConnMap[raddr.String()] = thisconn
					mutex.RUnlock()

					err = extractor.WriteUDPResponse(raddr, bs[:n])
					if err != nil {
						break
					}

				}
			}(newConn, addr)
		}

	}

}
