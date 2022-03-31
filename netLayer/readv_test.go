package netLayer

import (
	"bytes"
	"crypto/rand"
	"io"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/hahahrfool/v2ray_simple/utils"
)

/*
我们本地benchmark，实际benchmark readv 是比 经典拷贝慢的

BenchmarkReadVCopy-8                             	  426525	      2934 ns/op
BenchmarkClassicCopy-8                           	  531406	      2185 ns/op
BenchmarkClassicCopy_SimulateRealWorld-8         	   60873	     19631 ns/op
BenchmarkClassicCopy_SimulateRealWorld_ReadV-8   	   66138	     17907 ns/op


我们添加一种情况 SimulateRealWorld, 分10次写入, 每次写入长度均小于MTU，此时即可发现readv更快

总之这种本地benchmark 对于 readv来说意义不大，因为本地回环太快了, readv只能徒增各种附加操作.

理论上来说，在非本地测试环境下，只要每次传输的包超过了MTU，那么readv就应该是有优势的，包长度越大越有优势，因为越大越容易被割包，那么readv就越好用
*/

//我们不断向一个net.Conn 发送大数据
func TestReadVCopy(t *testing.T) {

	utils.InitLog()

	listenAddr := GetRandLocalAddr()
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	bigBytes := make([]byte, 10240) //10k

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != 10240 {
		t.Log(err)
		t.FailNow()
	}

	transmitCount := 10

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			//t.Log(err)
			t.Fail()
			return
		}

		for i := 0; i < transmitCount; i++ {
			buf := &bytes.Buffer{}
			_, err := TryCopyOnce(buf, conn)

			if err != nil {
				if strings.Contains(err.Error(), "close") {
					t.Fail()

				}
				//t.Log(err)
				return
			}

		}

	}()

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	for i := 0; i < transmitCount; i++ {
		_, e := tcpConn.Write(bigBytes)
		if e != nil {
			t.Log(err)
			t.FailNow()
		}
	}
	tcpConn.Close()
}

func BenchmarkReadVCopy(b *testing.B) {
	transmitCount := b.N

	listenAddr := GetRandLocalAddr()
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalln(err)
	}

	bigBytes := make([]byte, 10240) //10k

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != 10240 {
		log.Fatalln(n, err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			//b.Log(err)
			b.Fail()
		}
		//buf := &bytes.Buffer{}

		for i := 0; i < transmitCount; i++ {
			buf := utils.GetBuf()
			_, err := TryCopyOnce(buf, conn)

			if err != nil {
				if strings.Contains(err.Error(), "close") {
					b.Fail()

				}
			}
			utils.PutBuf(buf)
			//buf.Reset()

		}

	}()

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		log.Fatalln(err)
	}

	for i := 0; i < transmitCount; i++ {
		_, e := tcpConn.Write(bigBytes)
		if e != nil {
			log.Fatalln(err)
		}
	}
	tcpConn.Close()
}

func BenchmarkClassicCopy(b *testing.B) {

	b.StopTimer()
	b.ResetTimer()

	transmitCount := b.N

	listenAddr := GetRandLocalAddr()
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalln(err)
	}

	const bigBytesLen = 10240

	bigBytes := make([]byte, bigBytesLen) //10k

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != bigBytesLen {
		log.Fatalln(n, err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			//b.Log(err)
			b.Fail()
		}

		//bs := make([]byte, bigBytesLen)
		//buf := &bytes.Buffer{}
		// 不能直接使用单一buf，否则对readv来说不公平，必须同样从pool中存取

		for {

			buf := utils.GetBuf()
			//bs := utils.GetMTU()
			bs := utils.GetPacket()

			n, err := conn.Read(bs)

			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "close") {
					b.Fail()
				}
				return
			}
			buf.Write(bs[:n])
			utils.PutBuf(buf)

			utils.PutPacket(bs)
			//utils.PutBytes(bs)

			//buf.Reset()

		}

	}()

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		log.Fatalln(err)
	}
	b.StartTimer()

	for i := 0; i < transmitCount; i++ {
		_, e := tcpConn.Write(bigBytes)
		if e != nil {
			log.Fatalln(err)
		}
	}
	tcpConn.Close()
}

func BenchmarkClassicCopy_SimulateRealWorld(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	transmitCount := b.N

	listenAddr := GetRandLocalAddr()
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	const bigBytesLen = 10240

	bigBytes := make([]byte, bigBytesLen) //10k

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != bigBytesLen {
		b.Log(err)
		b.FailNow()
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			b.Log(err)
			b.Fail()
			return
		}

		//bs := make([]byte, bigBytesLen)

		for {
			//bs := utils.GetMTU()
			bs := utils.GetPacket()

			n, err := conn.Read(bs)

			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "close") {
					b.Log(err)
					b.Fail()

				}
				return
			}
			buf := utils.GetBuf()
			buf.Write(bs[:n])

			utils.PutBuf(buf)

			utils.PutPacket(bs)

			//utils.PutBytes(bs)

		}

	}()

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	b.StartTimer()

	for i := 0; i < transmitCount; i++ {
		unit := bigBytesLen / 10

		for cursor := 0; cursor < bigBytesLen; cursor += unit {
			_, e := tcpConn.Write(bigBytes[cursor : cursor+unit])
			if e != nil {
				//log.Fatalln(err)
				b.Log(err)
				b.FailNow()
			}
		}

	}
	tcpConn.Close()
}

func BenchmarkClassicCopy_SimulateRealWorld_ReadV(b *testing.B) {
	b.StopTimer()
	b.ResetTimer()

	transmitCount := b.N

	listenAddr := GetRandLocalAddr()
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}

	const bigBytesLen = 10240

	bigBytes := make([]byte, bigBytesLen) //10k

	n, err := rand.Reader.Read(bigBytes)
	if err != nil || n != bigBytesLen {
		b.Log(err)
		b.FailNow()
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			b.Log(err)
			b.Fail()
			return
		}

		for {

			//buf := &bytes.Buffer{}
			buf := utils.GetBuf()
			_, err := TryCopyOnce(buf, conn)

			if err != nil {
				if err != io.EOF && strings.Contains(err.Error(), "close") {
					b.Fail()

				}
				return
			}
			utils.PutBuf(buf)

		}

	}()

	tcpConn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		b.Log(err)
		b.FailNow()
	}
	unit := bigBytesLen / 10

	b.StartTimer()

	for i := 0; i < transmitCount; i++ {

		for cursor := 0; cursor < bigBytesLen; cursor += unit {
			_, e := tcpConn.Write(bigBytes[cursor : cursor+unit])
			if e != nil {
				//log.Fatalln(err)
				b.Log(err)
				b.FailNow()
			}
		}

	}
	tcpConn.Close()
}
