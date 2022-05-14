package proxy

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func TestTCP(protocol string, version int, port string, t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog("")

	url := protocol + "://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:" + port + "?version=" + strconv.Itoa(version)
	server, hase, _ := ServerFromURL(url)
	if hase {
		t.FailNow()
	}
	defer server.Stop()
	client, hase, _ := ClientFromURL(url)
	if hase {
		t.FailNow()
	}

	targetStr := "dummy.com:80"
	targetStruct := netLayer.Addr{
		Name: "dummy.com",
		Port: 80,
	}
	// 开始监听
	listener, err := net.Listen("tcp", server.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", server.AddrStr(), err)
		t.FailNow()
	}
	var testOK bool
	defer listener.Close()
	go func() {
		for {
			lc, err := listener.Accept()
			if err != nil && !testOK {
				t.Logf("failed in accept: %v", err)
				t.Fail()
				return
			}
			t.Log(protocol + " server got new conn")
			if lc == nil {
				return
			}
			go func() {
				defer lc.Close()
				wlc, _, targetAddr, err := server.Handshake(lc)
				if err != nil {
					t.Logf("failed in handshake form %v: %v", server.AddrStr(), err)
					t.Fail()
					return
				}

				if targetAddr.String() != targetStr {
					t.Log("fail x1")
					t.Fail()
					return
				}

				var hello [5]byte
				io.ReadFull(wlc, hello[:])
				if !bytes.Equal(hello[:], []byte("hello")) {
					t.Log("fail x2")
					t.Fail()
					return
				}

				wlc.Write([]byte("world"))
			}()
		}
	}()

	t.Log("client try dial ", server.AddrStr())
	// 连接
	rc, _ := net.Dial("tcp", server.AddrStr())
	defer rc.Close()

	t.Log("client Dial success")

	wrc, err := client.Handshake(rc, []byte("hello"), targetStruct)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", server.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client Handshake success")

	//wrc.Write([]byte("hello"))

	//t.Log("client write hello success")

	var world [5]byte
	n, err := io.ReadFull(wrc, world[:])
	if err != nil {
		t.Log("io.ReadFull(wrc, world[:])", err)
		t.FailNow()
	}
	t.Log("client read ok")

	if !bytes.Equal(world[:], []byte("world")) {
		t.Log("not equal", string(world[:]), world[:], n)
		t.FailNow()
	}

	t.Log("client match ok")

	testOK = true
}

// 完整模拟整个 protocol 的udp请求 过程，即 客户端连接代理服务器，代理服务器试图访问远程服务器，这里是使用的模拟的办法模拟出一个远程udp服务器；
// 其他tcp测试因为比较简单，不需要第二步测试，而这里需要
func TestUDP(protocol string, version int, proxyPort string, use_multi int, t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog("")

	t.Log("fakeServerEndLocalServer port is ", proxyPort)

	fmtStr := protocol + "://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:%s?version=%d&vless1_udp_multi=%d"

	url := fmt.Sprintf(fmtStr, proxyPort, version, use_multi)
	fakeServerEndLocalServer, hase, errx := ServerFromURL(url)
	if hase {
		t.Log("fakeClientEndLocalServer parse err", errx)
		t.FailNow()
	}
	defer fakeServerEndLocalServer.Stop()
	fakeClientEndRemoteClient, hase, errx := ClientFromURL(url)
	if hase {
		t.Log("fakeClientEndRemoteClient parse err", errx)
		t.FailNow()
	}

	fakeRealUDPServerPort := netLayer.RandPort(true, true)

	t.Log("fake remote udp server port is ", fakeRealUDPServerPort)

	fakeRealUDPServerListener, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: fakeRealUDPServerPort,
	})
	if err != nil {
		t.Log("监听失败 udp ", err)
		t.FailNow()
	}
	defer fakeRealUDPServerListener.Close()

	replydata := []byte("reply")
	hellodata := []byte("hello")

	//Fake UDP Server Goroutine
	go func() {
		readbuf := make([]byte, 10*1024)
		count := 0

		for {
			//t.Log(" udp for! ")
			// 读取数据
			readNum, remoteAddr, err := fakeRealUDPServerListener.ReadFromUDP(readbuf)
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					//t.Log("udp server read connection closed")
					return
				} else {
					t.Log("udp server 读取数据失败!", err)
				}

				//continue
				break
			}

			//首次包必须是hello，其他情况直接无视，直接返回 replydata
			if count == 0 {
				if readNum != 5 || !bytes.Equal(readbuf[:5], hellodata) {
					t.Log("udp read invalid data:", readbuf[:5], string(readbuf[:5]))
					t.Fail()
					return
				}
				t.Log(" udp 读到hello数据! ")
			}

			// 发送数据

			time.Sleep(time.Millisecond)
			_, err = fakeRealUDPServerListener.WriteToUDP(replydata, remoteAddr)
			if err != nil {
				t.Log("udp write back err:", err)
				t.Fail()
				return
			}
			//t.Log(" udp for! ", count, readNum)
			count++
		}

	}()

	targetStr_forFakeUDPServer := "127.0.0.1:" + strconv.Itoa(fakeRealUDPServerPort)
	targetStruct_forFakeUDPServer := netLayer.Addr{
		IP:      net.IPv4(127, 0, 0, 1),
		Port:    fakeRealUDPServerPort,
		Network: "udp",
	}
	// 监听 Client End LocalServer
	listener, err := net.Listen("tcp", fakeServerEndLocalServer.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}
	defer listener.Close()

	//一个完整的 protocol 服务端， 将客户端发来的udp数据转发到 目的地
	go func() {
		for {
			lc, err := listener.Accept()
			if err != nil {
				t.Logf("failed in accept: %v", err)
				break
			}
			go func() {
				defer lc.Close()
				_, wlc, targetAddr, err := fakeServerEndLocalServer.Handshake(lc)
				if err != nil {
					t.Logf("failed in handshake form %v: %v", fakeServerEndLocalServer.AddrStr(), err)
					t.Fail()
					return
				}

				remoteAddrStr := targetAddr.String()
				t.Log(protocol+" server got new wlc", remoteAddrStr)

				if remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.Network != "udp" {
					t.Log("remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.IsUDP == false ", remoteAddrStr, targetStr_forFakeUDPServer, targetAddr.Network)
					t.Fail()
					return
				}

				//这里的测试是，第一个发来的包必须是 hello，然后传递到目标udp服务器中

				//发现既可能读取 firstbuf，也可能读取 wlc，随机发生？

				t.Log(protocol + " read from wlc")
				bs, raddr, _ := wlc.ReadMsgFrom()

				t.Log(protocol+" got wlc", bs)

				if !bytes.Equal(bs, hellodata) {
					t.Log("!bytes.Equal(hello[:], hellodata)")
					t.Fail()
					return
				}

				t.Log(protocol + " got wlc with right hello data")

				t.Log(protocol+" server dialed remote udp server", remoteAddrStr)

				na, _ := netLayer.NewAddr(remoteAddrStr)
				na.Network = "udp"

				wrc, err := netLayer.NewUDPMsgConn(nil, false, false)
				if err != nil {
					t.Logf("failed netLayer.NewUDPMsgConn\n")
					t.Fail()
					return
				}

				err = wrc.WriteMsgTo(bs, na)
				if err != nil {
					t.Logf("failed wrc.WriteMsgTo : %v", err)
					t.Fail()
					return
				}

				bs, _, err = wrc.ReadMsgFrom()

				if err != nil {
					t.Logf("failed wrc.ReadMsgFrom : %v", err)
					t.Fail()
					return
				}

				err = wlc.WriteMsgTo(bs, raddr)
				if err != nil {
					t.Logf("failed wlc.WriteMsgTo : %v", err)
					t.Fail()
					return
				}

				// 之后转发所有流量，不再特定限制数据
				netLayer.RelayUDP(wrc, wlc, nil, nil)
				//t.Log("Copy End?!")
			}()
		}
	}()

	// 连接 Client End LocalServer
	rc, _ := net.Dial("tcp", fakeServerEndLocalServer.AddrStr())
	defer rc.Close()

	t.Log("client Dial success")

	wrc, err := fakeClientEndRemoteClient.EstablishUDPChannel(rc, nil, targetStruct_forFakeUDPServer)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client handshake success")

	err = wrc.WriteMsgTo(hellodata, targetStruct_forFakeUDPServer)
	if err != nil {
		t.Log("failed in write to ", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client write hello success")

	bs, _, err := wrc.ReadMsgFrom()
	if !bytes.Equal(bs, replydata) {
		t.Log("!bytes.Equal(world[:], replydata) ", bs, replydata, err)
		t.FailNow()
	}
	t.Log("读到正确reply！")

	//再尝试 发送 长信息，确保  实现 对于长信息来说 依然 没有问题

	longbs := make([]byte, 9*1024)

	for i := 0; i < 10; i++ {

		//目前实测，9*1024是好使的，但是9*1025 以上就会出问题？？一旦增加，测试就会卡住
		// 可能是udp传输时卡住了, 因为长度太大导致丢包, 服务端没有收到此包，我们就收不到服务端发来的回应，就会卡住.
		// 总之这个和udp性质有关。我们传输udp时不要发过大的包，或不要过快发送即可。否则就要有防丢包措施；
		// 不能期待服务器一定会收到udp消息 而陷入 对服务器回应的无限等待中

		rand.Reader.Read(longbs)

		t.Log("rand generated", len(longbs), longbs[:5])

		err = wrc.WriteMsgTo(longbs, targetStruct_forFakeUDPServer)
		if err != nil {
			t.Log("failed in write long data to ", fakeServerEndLocalServer.AddrStr(), err)
			t.FailNow()
		}

		t.Log("data written")

		bs, _, _ := wrc.ReadMsgFrom()
		if err != nil {
			t.Log("ReadFull err ", err)
			t.FailNow()
		}

		t.Log("data read complete")

		if !bytes.Equal(bs, replydata) {
			t.Logf("reply not equal %q %q\n", string(replydata), string(bs))
			t.FailNow()
		}
		t.Log("compare success")

	}

}
