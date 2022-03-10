package vless

import (
	"bytes"
	"crypto/rand"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/hahahrfool/v2ray_simple/proxy"
)

func TestVLess0(t *testing.T) {
	testVLess(0, "9527", t)
}

func TestVLess1(t *testing.T) {
	testVLess(1, "9538", t)
}

func testVLess(version int, port string, t *testing.T) {
	url := "vless://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:" + port + "?version=" + strconv.Itoa(version)
	server, err := proxy.ServerFromURL(url)
	if err != nil {
		t.FailNow()
	}
	defer server.Stop()
	client, err := proxy.ClientFromURL(url)
	if err != nil {
		t.FailNow()
	}

	targetStr := "dummy.com:80"
	targetStruct := &proxy.Addr{
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
			go func() {
				defer lc.Close()
				wlc, targetAddr, err := server.Handshake(lc)
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
	testVLessUDP(0, "9638", t)
}

//func TestVLess1_udp(t *testing.T) {
//testVLessUDP(1, "9738", t)	//无法使用 testVLessUDP，见其注释
//}

// 完整模拟整个 vless v0 的udp请求 过程，即 客户端连接代理服务器，代理服务器试图访问远程服务器，这里是使用的模拟的办法模拟出一个远程udp服务器；
// 其他tcp测试因为比较简单，不需要第二步测试，而这里需要
//  不过实测，这个test暂时只能使用v0版本，因为 v1版本具有 独特信道，不能直接使用下面代码。
func testVLessUDP(version int, port string, t *testing.T) {
	url := "vless://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:" + port + "?version=" + strconv.Itoa(version)
	fakeServerEndLocalServer, err := proxy.ServerFromURL(url)
	if err != nil {
		t.Log("fakeClientEndLocalServer parse err", err)
		t.FailNow()
	}
	defer fakeServerEndLocalServer.Stop()
	fakeClientEndRemoteClient, err := proxy.ClientFromURL(url)
	if err != nil {
		t.Log("fakeClientEndRemoteClient parse err", err)
		t.FailNow()
	}

	thePort := 30002

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
			t.Log(" udp for! ")
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
			//break
			t.Log(" udp for! ", count, readNum)
			count++
		}

	}()

	targetStr_forFakeUDPServer := "127.0.0.1:" + strconv.Itoa(thePort)
	targetStruct_forFakeUDPServer := &proxy.Addr{
		Name:  "127.0.0.1",
		Port:  thePort,
		IsUDP: true,
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
				wlc, targetAddr, err := fakeServerEndLocalServer.Handshake(lc)
				if err != nil {
					t.Logf("failed in handshake form %v: %v", fakeServerEndLocalServer.AddrStr(), err)
					t.Fail()
					return
				}

				remoteAddrStr := targetAddr.String()

				if remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.IsUDP == false {
					t.Log("remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.IsUDP == false ")
					t.Fail()
					return
				}

				rc, err := net.Dial("udp", remoteAddrStr)
				if err != nil {
					t.Logf("failed to connect FakeUDPServer : %v", err)
					t.Fail()
					return
				}

				//这里的测试是，第一个发来的包必须是 hello，然后传递到目标udp服务器中

				var hello [5]byte
				io.ReadFull(wlc, hello[:])
				if !bytes.Equal(hello[:], hellodata) {
					t.Log("!bytes.Equal(hello[:], hellodata)")
					t.Fail()
					return
				}
				_, err = rc.Write(hello[:])
				if err != nil {
					t.Logf("failed to write to FakeUDPServer : %v", err)
					t.Fail()
					return
				}
				_, err = io.ReadFull(rc, hello[:])
				if err != nil {
					t.Logf("failed io.ReadFull(rc, hello[:]) : %v", err)
					t.Fail()
					return
				}

				_, err = wlc.Write(hello[:])
				if err != nil {
					t.Logf("failed wlc.Write(hello[:]) : %v", err)
					t.Fail()
					return
				}

				// 之后转发所有流量，不再特定限制数据
				go io.Copy(rc, wlc)
				_, err = io.Copy(wlc, rc)

				t.Log("Copy End?!", err)
			}()
		}
	}()

	// 连接 Client End LocalServer
	rc, _ := net.Dial("tcp", fakeServerEndLocalServer.AddrStr())
	defer rc.Close()

	t.Log("client Dial success")

	wrc, err := fakeClientEndRemoteClient.Handshake(rc, targetStruct_forFakeUDPServer)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client vless handshake success")

	_, err = wrc.Write(hellodata)
	if err != nil {
		t.Log("failed in write to ", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}

	t.Log("client write hello success")

	var world [5]byte
	io.ReadFull(wrc, world[:])
	if !bytes.Equal(world[:], replydata) {
		t.Log("!bytes.Equal(world[:], replydata) ", world[:], replydata)
		t.FailNow()
	}
	t.Log("读到正确reply！")

	//再试图发送长信息，确保 vless v0 的实现没有问题

	for i := 0; i < 10; i++ {
		longbs := make([]byte, 9*1024) //目前实测，9*1024是好使的，但是9*1025 以上就会出问题？？

		rand.Reader.Read(longbs)

		t.Log("rand generated", len(longbs))

		_, err = wrc.Write(longbs)
		if err != nil {
			t.Log("failed in write long data to ", fakeServerEndLocalServer.AddrStr(), err)
			t.FailNow()
		}

		t.Log("data written")

		var world [5]byte
		n, err := io.ReadFull(wrc, world[:])
		if err != nil {
			t.Log("ReadFull err ", n, err)
			t.FailNow()
		}

		t.Log("data read complete")

		if !bytes.Equal(world[:], replydata) {
			t.Log("reply not equal ", string(replydata), string(world[:]))
			t.FailNow()
		}
		t.Log("compare success")

	}

}
