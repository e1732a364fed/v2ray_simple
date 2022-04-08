package vless_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestVLess0(t *testing.T) {
	testVLess(0, netLayer.RandPortStr(), t)
}

func TestVLess1(t *testing.T) {
	testVLess(1, netLayer.RandPortStr(), t)
}

func testVLess(version int, port string, t *testing.T) {
	url := "vless://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:" + port + "?version=" + strconv.Itoa(version)
	server, hase, _ := proxy.ServerFromURL(url)
	if hase {
		t.FailNow()
	}
	defer server.Stop()
	client, hase, _ := proxy.ClientFromURL(url)
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
	go func() {
		for {
			lc, err := listener.Accept()
			if err != nil {
				t.Logf("failed in accept: %v", err)
				t.Fail()
			}
			t.Log("vless server got new conn")
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
				//io.ReadFull(wlc, hello[:])
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

	wrc, err := client.Handshake(rc, targetStruct)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", server.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client vless Handshake success")

	wrc.Write([]byte("hello"))

	t.Log("client write hello success")

	var world [5]byte
	n, err := io.ReadFull(wrc, world[:])
	if err != nil {
		t.Log("io.ReadFull(wrc, world[:])", err)
		t.FailNow()
	}

	if !bytes.Equal(world[:], []byte("world")) {
		t.Log("not equal", string(world[:]), world[:], n)
		t.FailNow()
	}
}

func TestVLess0_udp(t *testing.T) {
	testVLessUDP(0, netLayer.RandPortStr(), t)
}

//func TestVLess1_udp(t *testing.T) {
//testVLessUDP(1, "9738", t)	//无法使用 testVLessUDP，见其注释
//}

// 完整模拟整个 vless v0 的udp请求 过程，即 客户端连接代理服务器，代理服务器试图访问远程服务器，这里是使用的模拟的办法模拟出一个远程udp服务器；
// 其他tcp测试因为比较简单，不需要第二步测试，而这里需要
//  不过实测，这个test暂时只能使用v0版本，因为 v1版本具有 独特信道，不能直接使用下面代码。
func testVLessUDP(version int, port string, t *testing.T) {
	url := "vless://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:" + port + "?version=" + strconv.Itoa(version)
	fakeServerEndLocalServer, hase, errx := proxy.ServerFromURL(url)
	if hase {
		t.Log("fakeClientEndLocalServer parse err", errx)
		t.FailNow()
	}
	defer fakeServerEndLocalServer.Stop()
	fakeClientEndRemoteClient, hase, errx := proxy.ClientFromURL(url)
	if hase {
		t.Log("fakeClientEndRemoteClient parse err", errx)
		t.FailNow()
	}

	thePort := netLayer.RandPort()

	t.Log("fake remote udp server port is ", thePort)

	fakeRealUDPServerListener, err := net.ListenUDP("udp4", &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: thePort,
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
					t.Log("udp server read connection closed")
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

	targetStr_forFakeUDPServer := "127.0.0.1:" + strconv.Itoa(thePort)
	targetStruct_forFakeUDPServer := netLayer.Addr{
		IP:      net.IPv4(127, 0, 0, 1),
		Port:    thePort,
		Network: "udp",
	}
	// 监听 Client End LocalServer
	listener, err := net.Listen("tcp", fakeServerEndLocalServer.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	//一个完整的 vless 服务端， 将客户端发来的数据转发到 目的地
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
				t.Log("vless server got new wlc", remoteAddrStr)

				if remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.Network != "udp" {
					t.Log("remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.IsUDP == false ", remoteAddrStr, targetStr_forFakeUDPServer, targetAddr.Network)
					t.Fail()
					return
				}

				//这里的测试是，第一个发来的包必须是 hello，然后传递到目标udp服务器中

				//发现既可能读取 firstbuf，也可能读取 wlc，随机发生？

				t.Log("vless read from wlc")
				bs, raddr, _ := wlc.ReadFrom()

				t.Log("vless got wlc", bs)

				if !bytes.Equal(bs, hellodata) {
					t.Log("!bytes.Equal(hello[:], hellodata)")
					t.Fail()
					return
				}

				t.Log("vless got wlc with right hello data")

				rc, err := net.Dial("udp", remoteAddrStr)
				if err != nil {
					t.Logf("failed to connect FakeUDPServer : %v", err)
					t.Fail()
					return
				}

				t.Log("vless server dialed remote udp server", remoteAddrStr)

				na, _ := netLayer.NewAddr(remoteAddrStr)
				na.Network = "udp"

				wrc := &netLayer.UDPMsgConnWrapper{UDPConn: rc.(*net.UDPConn), IsClient: true, FirstAddr: na}

				_, err = rc.Write(bs)
				if err != nil {
					t.Logf("failed to write to FakeUDPServer : %v", err)
					t.Fail()
					return
				}
				_, err = io.ReadFull(rc, bs)
				if err != nil {
					t.Logf("failed io.ReadFull(rc, hello[:]) : %v", err)
					t.Fail()
					return
				}

				err = wlc.WriteTo(bs, raddr)
				if err != nil {
					t.Logf("failed wlc.Write(hello[:]) : %v", err)
					t.Fail()
					return
				}

				// 之后转发所有流量，不再特定限制数据
				netLayer.RelayUDP(wlc, wrc)

				t.Log("Copy End?!", err)
			}()
		}
	}()

	// 连接 Client End LocalServer
	rc, _ := net.Dial("tcp", fakeServerEndLocalServer.AddrStr())
	defer rc.Close()

	t.Log("client Dial success")

	wrc, err := fakeClientEndRemoteClient.EstablishUDPChannel(rc, targetStruct_forFakeUDPServer)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client vless handshake success")

	err = wrc.WriteTo(hellodata, targetStruct_forFakeUDPServer)
	if err != nil {
		t.Log("failed in write to ", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client write hello success")

	bs, _, _ := wrc.ReadFrom()
	if !bytes.Equal(bs, replydata) {
		t.Log("!bytes.Equal(world[:], replydata) ", bs, replydata)
		t.FailNow()
	}
	t.Log("读到正确reply！")

	//再试图发送长信息，确保 vless v0 的实现没有问题

	for i := 0; i < 10; i++ {
		longbs := make([]byte, 9*1024)

		//目前实测，9*1024是好使的，但是9*1025 以上就会出问题？？一旦增加，测试就会卡住
		// 可能是udp传输时卡住了, 因为长度太大导致丢包, 服务端没有收到此包，我们就收不到服务端发来的回应，就会卡住.
		// 总之这个和udp性质有关。我们传输udp时不要发过大的包，或者不要过快发送即可。否则就要有防丢包措施；
		// 不能期待服务器一定会收到udp消息 而陷入 对服务器回应的无限等待中

		rand.Reader.Read(longbs)

		t.Log("rand generated", len(longbs))

		err = wrc.WriteTo(longbs, targetStruct_forFakeUDPServer)
		if err != nil {
			t.Log("failed in write long data to ", fakeServerEndLocalServer.AddrStr(), err)
			t.FailNow()
		}

		t.Log("data written")

		bs, _, _ := wrc.ReadFrom()
		if err != nil {
			t.Log("ReadFull err ", err)
			t.FailNow()
		}

		t.Log("data read complete")

		if !bytes.Equal(bs, replydata) {
			t.Log("reply not equal ", string(replydata), string(bs))
			t.FailNow()
		}
		t.Log("compare success")

	}

}
