package vmess

import (
	"crypto/cipher"
	"crypto/rand"
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

	shakeParser *ShakeSizeParser
}

func AEADWriter(w io.Writer, aead cipher.AEAD, iv []byte, shakeParser *ShakeSizeParser) io.Writer {
	return &aeadWriter{
		Writer:      w,
		AEAD:        aead,
		buf:         utils.GetPacket(), //make([]byte, lenSize+chunkSize),
		nonce:       make([]byte, aead.NonceSize()),
		iv:          iv,
		shakeParser: shakeParser,
	}
}

func (w *aeadWriter) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return
	}

	if w.shakeParser != nil {

		encryptedSize := (len(b) + w.Overhead())
		paddingSize := int(w.shakeParser.NextPaddingLen())
		sizeBytes := 2
		totalSize := 2 + encryptedSize + paddingSize

		eb := w.buf[:totalSize]

		w.shakeParser.Encode(uint16(encryptedSize+paddingSize), eb[:sizeBytes])
		encryptBuf := eb[sizeBytes : sizeBytes+encryptedSize]

		binary.BigEndian.PutUint16(w.nonce[:2], w.count)
		copy(w.nonce[2:], w.iv[2:12])

		w.Seal(encryptBuf[:0], w.nonce, b, nil)
		w.count++

		if paddingSize > 0 {
			rand.Read(eb[sizeBytes+encryptedSize:])
		}

		_, err = w.Writer.Write(eb)
		n = len(b)

	} else {
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
	}

	return
}

type aeadReader struct {
	io.Reader
	cipher.AEAD
	nonce       []byte
	buf         []byte
	leftover    []byte
	count       uint16
	iv          []byte
	shakeParser *ShakeSizeParser
	done        bool
}

func AEADReader(r io.Reader, aead cipher.AEAD, iv []byte, shakeParser *ShakeSizeParser) io.Reader {
	return &aeadReader{
		Reader:      r,
		AEAD:        aead,
		buf:         utils.GetPacket(),
		nonce:       make([]byte, aead.NonceSize()),
		iv:          iv,
		shakeParser: shakeParser,
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
	var padding uint16
	var err error

	if r.shakeParser == nil {

		_, err = io.ReadFull(r.Reader, r.buf[:lenSize])
		if err != nil {
			return 0, err
		}

		l = binary.BigEndian.Uint16(r.buf[:lenSize])
	} else {
		//顺序不要搞错，要先 读padding，然后再 shake 长度，否则会出错. 实测v2ray的vmess的padding默认就是开启状态
		padding = r.shakeParser.NextPaddingLen()

		var sbA [2]byte
		sb := sbA[:]

		if _, err = io.ReadFull(r.Reader, sb); err != nil {
			return 0, err
		}
		l, err = r.shakeParser.Decode(sb)
		if err != nil {
			return 0, err
		}

		if l == uint16(r.AEAD.Overhead())+padding {
			r.done = true
			return 0, io.EOF
		}

	}

	if l == 0 {
		return 0, nil
	}
	if l > chunkSize && r.shakeParser == nil {
		return 0, fmt.Errorf("l>chunkSize(16k), %d", l) //有可能出现这种情况
	}

	// get payload
	buf := r.buf[:l]

	_, err = io.ReadFull(r.Reader, buf)
	if err != nil {

		return 0, err
	}

	if r.shakeParser != nil {
		buf = buf[:int(l)-int(padding)]
	}

	binary.BigEndian.PutUint16(r.nonce[:2], r.count)
	copy(r.nonce[2:], r.iv[2:12])

	returnedData, err := r.Open(buf[:0], r.nonce, buf, nil)
	r.count++
	if err != nil {
		return 0, err
	}

	var dataLen int

	if r.shakeParser == nil {
		dataLen = int(l) - r.Overhead()

	} else {
		dataLen = len(returnedData)

	}

	m := copy(b, buf[:dataLen])
	if m < int(dataLen) {
		r.leftover = buf[m:dataLen]
	}

	return m, err
}
