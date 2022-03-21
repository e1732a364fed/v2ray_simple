package ws

import (
	"bytes"
	"crypto/rand"
	"net"
	"testing"
)

// 分别测试写入短数据和长数据
func TestWs(t *testing.T) {
	listenAddr := "127.0.0.1:7777"
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	wsPath := "/thepath"

	bigBytes := make([]byte, 10240)

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != 10240 {
		t.Log(err)
		t.FailNow()
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}

		wsConn, err := Handshake(wsPath, conn)
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
		bs := make([]byte, 1500)
		msgCount := 0
		for {
			n, err := wsConn.Read(bs)
			if err != nil {
				t.Log(err)
				t.Fail()
				return
			}
			nbs := bs[:n]
			t.Log("listener got", n, string(nbs))
			if msgCount == 0 {
				wsConn.Write([]byte("world"))

			} else {
				wsConn.Write(bigBytes)
			}
			msgCount++
		}
	}()

	cli, err := NewClient(listenAddr, wsPath)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	wsConn, err := cli.Handshake(tcpConn)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	_, err = wsConn.Write([]byte("hello"))
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	bs := make([]byte, 15000)
	n, err = wsConn.Read(bs)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	t.Log("client got", n, string(bs[:n]))

	_, err = wsConn.Write([]byte("hello2"))
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	n, err = wsConn.Read(bs)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	if n != len(bigBytes) || !bytes.Equal(bs[:n], bigBytes) {
		t.Log("not equal", n)
		t.FailNow()
	}
}
