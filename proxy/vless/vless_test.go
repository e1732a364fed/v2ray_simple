package vless

import (
	"bytes"
	"io"
	"log"
	"net"
	"strconv"
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
				}

				if targetAddr.String() != targetStr {
					t.Fail()
				}

				var hello [5]byte
				io.ReadFull(wlc, hello[:])
				if !bytes.Equal(hello[:], []byte("hello")) {
					t.Fail()
				}

				wlc.Write([]byte("world"))
			}()
		}
	}()

	// 连接
	rc, _ := net.Dial("tcp", server.AddrStr())
	defer rc.Close()

	wrc, err := client.Handshake(rc, targetStruct)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", server.AddrStr(), err)
		t.FailNow()
	}
	wrc.Write([]byte("hello"))

	var world [5]byte
	io.ReadFull(wrc, world[:])
	if !bytes.Equal(world[:], []byte("world")) {
		t.FailNow()
	}
}

func TestVLessUDP(t *testing.T) {
	url := "vless://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:9528"
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
		readbuf := make([]byte, 5)

		for {
			t.Log(" udp for! ")
			// 读取数据
			readNum, remoteAddr, err := fakeRealUDPServerListener.ReadFromUDP(readbuf)
			if err != nil {
				t.Log("udp server 读取数据失败!", err)
				//continue
				break
			}

			if readNum != 5 || !bytes.Equal(readbuf, hellodata) {
				t.Log("udp read invalid data:", readbuf, string(readbuf))
				t.Fail()
			}
			t.Log(" udp 读到hello数据! ")

			// 发送数据

			_, err = fakeRealUDPServerListener.WriteToUDP(replydata, remoteAddr)
			if err != nil {
				t.Log("udp write back err:", err)
				t.Fail()
			}
			break
		}

	}()

	targetStr_forFakeUDPServer := "127.0.0.1:" + strconv.Itoa(thePort)
	targetStruct_forFakeUDPServer := &proxy.Addr{
		Name:  "127.0.0.1",
		Port:  thePort,
		IsUDP: true,
	}
	// 开始监听 Client End LocalServer
	listener, err := net.Listen("tcp", fakeServerEndLocalServer.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}
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
				}

				remoteAddrStr := targetAddr.String()

				if remoteAddrStr != targetStr_forFakeUDPServer || targetAddr.IsUDP == false {
					t.Fail()
				}

				rc, err := net.Dial("udp", remoteAddrStr)
				if err != nil {
					t.Logf("failed to connect FakeUDPServer : %v", err)
					t.Fail()
				}

				var hello [5]byte
				io.ReadFull(wlc, hello[:])
				if !bytes.Equal(hello[:], hellodata) {
					t.Fail()
				}
				_, err = rc.Write(hello[:])
				if err != nil {
					t.Logf("failed to write to FakeUDPServer : %v", err)
					t.Fail()
				}
				io.ReadFull(rc, hello[:])

				wlc.Write(hello[:])
			}()
		}
	}()

	// 连接 Client End LocalServer
	rc, _ := net.Dial("tcp", fakeServerEndLocalServer.AddrStr())
	defer rc.Close()

	wrc, err := fakeClientEndRemoteClient.Handshake(rc, targetStruct_forFakeUDPServer)
	if err != nil {
		log.Printf("failed in handshake to %v: %v", fakeServerEndLocalServer.AddrStr(), err)
		t.FailNow()
	}
	wrc.Write(hellodata)

	var world [5]byte
	io.ReadFull(wrc, world[:])
	if !bytes.Equal(world[:], replydata) {
		t.FailNow()
	}
	t.Log("读到正确reply！")
}
