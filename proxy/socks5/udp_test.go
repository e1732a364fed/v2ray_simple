package socks5_test

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy/direct"
	"github.com/hahahrfool/v2ray_simple/proxy/socks5"
)

//tcp就不测了，我们实践直接测试完全好使，这里重点测试UDP
// 因为chrome也是无法通过 socks5去申请udp链接的，所以没法自己用浏览器测试

//下面的部分代码在 main.go 中也有用到.
func TestUDP(t *testing.T) {

	s := &socks5.Server{}

	//建立socks5服务并监听，这里仅用于 udp associate 握手
	sAddrStr := netLayer.GetRandLocalAddr()
	listener, err := net.Listen("tcp", sAddrStr)
	if err != nil {
		t.Log("can not listen on", sAddrStr, err)
		t.FailNow()
	}

	direct, _ := direct.NewClient()

	go func() {
		for {
			lc, err := listener.Accept()
			if err != nil {
				t.Logf("failed in accept: %v", err)
				t.Fail()
			}
			t.Log("socks5 server got new conn")
			_, wlc, targetAddr, err := s.Handshake(lc)
			if targetAddr.IsUDP() {
				t.Log("socks5 server got udp associate")
			}
			//此时wlc返回的是socks5新监听的 conn

			go func() {
				for {
					t.Log("socks5 server start read udp channel")

					bs, addr, err := wlc.ReadFrom()
					if err != nil {
						t.Log("socks5 server read udp channel err,", err)

						break
					}

					t.Log("socks5 server got udp msg")

					msgConn, err := direct.EstablishUDPChannel(nil, addr)
					if err != nil {
						t.Fail()
						return
					}
					err = msgConn.WriteTo(bs, addr)
					if err != nil {
						t.Log("socks5 server Write To direct failed,", len(bs), err)
					}
					go func() {
						for {
							rbs, raddr, err := msgConn.ReadFrom()
							if err != nil {
								break
							}
							wlc.WriteTo(rbs, raddr)
						}
					}()
				}
			}()

		}
	}()

	//建立虚拟目标udp服务器并监听
	fakeUDP_ServerPort := netLayer.RandPort()

	fakeRealUDPServerListener, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: fakeUDP_ServerPort,
	})
	if err != nil {
		t.Log("监听失败 udp ", err)
		t.FailNow()
	}
	defer fakeRealUDPServerListener.Close()

	go func() {
		readbuf := make([]byte, 10*1024)

		for {
			t.Log(" udp for! ")
			// 读取数据, 无视任何信息，直接返回 "reply"
			n, remoteAddr, err := fakeRealUDPServerListener.ReadFromUDP(readbuf)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					t.Log("udp server read connection closed")
					return
				} else {
					t.Log("udp server 读取数据失败!", err)
				}

				//continue
				break
			}
			t.Log("fake udp server got", remoteAddr, string(readbuf[:n]))

			_, err = fakeRealUDPServerListener.WriteToUDP([]byte("reply"), remoteAddr)
			if err != nil {
				t.Log("udp write back err:", err)
				t.Fail()
				return
			}

			t.Log("fake udp server written")

		}
	}()

	//尝试客户端 UDPAssociate 发起请求
	rc, _ := net.Dial("tcp", sAddrStr)
	defer rc.Close()

	socks5_ServerPort, err := socks5.Client_EstablishUDPAssociate(rc)
	if err != nil {
		t.Log("Client_EstablishUDPAssociate failed", err)
		t.FailNow()
	}
	t.Log("Server Port", socks5_ServerPort)

	//获知 服务器udp端口后，再向该端口发起请求

	raSocks5, _ := netLayer.NewAddr("127.0.0.1:" + strconv.Itoa(socks5_ServerPort))

	raSocks5UDPAddr := raSocks5.ToUDPAddr()

	urc, err := net.DialUDP("udp", nil, raSocks5UDPAddr)
	if err != nil {
		t.Log("DialUDP failed", raSocks5UDPAddr)
		t.FailNow()
	}
	defer urc.Close()

	raFake, _ := netLayer.NewAddr("127.0.0.1:" + strconv.Itoa(fakeUDP_ServerPort))

	t.Log("call Client_RequestUDP")

	socks5.Client_RequestUDP(urc, &raFake, []byte("hello"))

	t.Log("call Client_ReadUDPResponse")

	ta, data, err := socks5.Client_ReadUDPResponse(urc, raSocks5UDPAddr)
	if err != nil {
		t.Log("Client_ReadUDPResponse failed", err)
		t.FailNow()
	}
	t.Log(ta, string(data))
}
