package tlsLayer_test

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vless"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

func TestVlesss(t *testing.T) {
	testTls("vlesss", t)
}

func testTls(protocol string, t *testing.T) {
	utils.LogLevel = utils.Log_debug
	utils.InitLog("")

	port := netLayer.RandPortStr(true, false)

	url := protocol + "://a684455c-b14f-11ea-bf0d-42010aaa0003@localhost:" + port + "?alterID=4&cert=../cert.pem&key=../cert.key&insecure=1"
	server, errx := proxy.ServerFromURL(url)
	if errx != nil {
		t.Log("fail1", errx)
		t.FailNow()
	}
	defer server.Stop()
	client, errx := proxy.ClientFromURL(url)
	if errx != nil {
		t.Log("fail2", errx)
		t.FailNow()
	}

	targetStr := "dummy.com:80"
	targetStruct := netLayer.Addr{
		Name: "dummy.com",
		Port: 80,
	}

	listener, err := net.Listen("tcp", server.AddrStr())
	if err != nil {
		t.Logf("can not listen on %v: %v", server.AddrStr(), err)
		t.FailNow()
	}
	defer listener.Close()
	go func() {

		lc, err := listener.Accept()
		if err != nil {
			t.Logf("failed in accept: %v", err)
			t.Fail()
			return
		}
		go func() {
			defer lc.Close()

			t.Log("server got new Conn")

			tlsConn, err := server.GetTLS_Server().Handshake(lc)
			if err != nil {
				t.Log("failed in server tls handshake  ", err)
				t.Fail()
				return
			}
			lc = tlsConn

			t.Log("server pass tls handshake")

			wlc, _, targetAddr, err := server.Handshake(lc)
			if err != nil {
				t.Log("failed in handshake from ", server.AddrStr(), err)
				t.Fail()
				return
			}

			t.Log("server pass vless handshake")

			if targetAddr.String() != targetStr {
				t.Fail()
				return
			}

			var hello [5]byte
			io.ReadFull(wlc, hello[:])
			if !bytes.Equal(hello[:], []byte("hello")) {
				t.Fail()
				return
			}

			wlc.Write([]byte("world"))
		}()

	}()

	t.Log("client dial ")

	rc, _ := net.Dial("tcp", server.AddrStr())
	defer rc.Close()

	t.Log("client handshake tls ")

	tlsConn, err := client.GetTLS_Client().Handshake(rc)
	if err != nil {
		t.Log("failed in client tls handshake  ", err)
		t.FailNow()
	}
	rc = tlsConn

	t.Log("client handshake vless ")

	wrc, err := client.Handshake(rc, []byte("hello"), targetStruct)
	if err != nil {
		t.Log("failed in handshake to", server.AddrStr(), err)
		t.FailNow()
	}

	//t.Log("client write hello ")

	//wrc.Write([]byte("hello"))

	t.Log("client read response ")
	var world [5]byte
	io.ReadFull(wrc, world[:])
	if !bytes.Equal(world[:], []byte("world")) {
		t.FailNow()
	}
}
