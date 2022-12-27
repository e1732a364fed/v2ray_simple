package vmess

import (
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

type aeadWriter struct {
	io.Writer
	cipher.AEAD
	nonce []byte
	buf   []byte
	count uint16
	iv    []byte
}

func AEADWriter(w io.Writer, aead cipher.AEAD, iv []byte) io.Writer {
	return &aeadWriter{
		Writer: w,
		AEAD:   aead,
		buf:    utils.GetPacket(), //make([]byte, lenSize+chunkSize),
		nonce:  make([]byte, aead.NonceSize()),
		iv:     iv,
	}
}

func (w *aeadWriter) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return
	}

	buf := w.buf
	//这里默认len(b)不大于 64k, 否则会闪退; 不过因为本作所有缓存最大就是64k，所以应该是不会出现问题的，所以也不加判断了。
	n = len(b)
	buf = buf[:lenSize+n+w.Overhead()]

	payloadBuf := buf[lenSize : lenSize+n]
	binary.BigEndian.PutUint16(buf[:lenSize], uint16(n+w.Overhead()))

	binary.BigEndian.PutUint16(w.nonce[:2], w.count)
	copy(w.nonce[2:], w.iv[2:12])

	w.Seal(payloadBuf[:0], w.nonce, b, nil)
	w.count++

	_, err = w.Writer.Write(buf)

	return
}

type aeadReader struct {
	io.Reader
	cipher.AEAD
	nonce    []byte
	buf      []byte
	leftover []byte
	count    uint16
	iv       []byte
	done     bool
}

func AEADReader(r io.Reader, aead cipher.AEAD, iv []byte) io.Reader {
	return &aeadReader{
		Reader: r,
		AEAD:   aead,
		buf:    utils.GetPacket(),
		nonce:  make([]byte, aead.NonceSize()),
		iv:     iv,
	}
}

func (r *aeadReader) Read(b []byte) (int, error) {

	if len(r.leftover) > 0 {
		n := copy(b, r.leftover)
		r.leftover = r.leftover[n:]
		return n, nil
	}

	if r.done {
		return 0, io.EOF
	}

	// get length

	var l uint16
	// var padding uint16
	var err error

	_, err = io.ReadFull(r.Reader, r.buf[:lenSize])
	if err != nil {
		return 0, err
	}

	l = binary.BigEndian.Uint16(r.buf[:lenSize])

	if l == 0 {
		return 0, nil
	}
	if l > chunkSize { // && r.shakeParser == nil
		return 0, fmt.Errorf("l>chunkSize(16k), %d", l) //有可能出现这种情况
	}

	// get payload
	buf := r.buf[:l]

	_, err = io.ReadFull(r.Reader, buf)
	if err != nil {

		return 0, err
	}

	binary.BigEndian.PutUint16(r.nonce[:2], r.count)
	copy(r.nonce[2:], r.iv[2:12])

	_, err = r.Open(buf[:0], r.nonce, buf, nil)
	r.count++
	if err != nil {
		return 0, err
	}

	dataLen := int(l) - r.Overhead()

	m := copy(b, buf[:dataLen])
	if m < int(dataLen) {
		r.leftover = buf[m:dataLen]
	}

	return m, err
}
