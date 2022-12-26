package vmess

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/sha3"
)

const (
	kdfSaltConstAEADIV  = "AEAD Nonce"
	kdfSaltConstAEADKEY = "AEAD KEY"
)

const (
	authid_len              = 16
	authID_timeMaxSecondGap = 120
)

var ErrAuthID_timeBeyondGap = utils.ErrInErr{ErrDesc: fmt.Sprintf("vmess: time gap more than %d second", authID_timeMaxSecondGap), ErrDetail: utils.ErrInvalidData}

func kdf(key []byte, info ...[]byte) []byte {
	h := sha3.New512()
	h.Write(key)
	for _, v := range info {
		h.Write(v)
	}
	h.Write(key)
	result := h.Sum(nil)
	h.Reset()
	return result
}

// https://github.com/v2fly/v2fly-github-io/issues/20
func createAuthID(key []byte, time int64) (result []byte) {
	buf := utils.GetBuf()
	defer utils.PutBuf(buf)
	binary.Write(buf, binary.BigEndian, time)
	timekey := kdf(key, []byte(kdfSaltConstAEADKEY), []byte("time"))
	aead, err := chacha20poly1305.New(timekey[:chacha20poly1305.KeySize])
	if err != nil {
		return
	}
	return aead.Seal(nil, timekey[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()], buf.Bytes(), nil)
}

// key长度必须16位
func sealAEADHeader(key []byte, data []byte, t time.Time) []byte {
	generatedAuthID := createAuthID(key[:], t.Unix())

	HdrLengthSerializedByte := make([]byte, 2)
	binary.BigEndian.PutUint16(HdrLengthSerializedByte, uint16(len(data)))

	key = kdf(key, []byte(kdfSaltConstAEADKEY), []byte("HeaderLength"))
	aead, err := chacha20poly1305.New(key[:chacha20poly1305.KeySize])
	if err != nil {
		return []byte("Fail to aead")
	}
	HdrLengthAEAD := aead.Seal(nil, key[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()], HdrLengthSerializedByte, nil)

	key = kdf(key, []byte(kdfSaltConstAEADKEY), []byte("Header"))
	aead, err = chacha20poly1305.New(key[:chacha20poly1305.KeySize])
	if err != nil {
		return []byte("Fail to aead")
	}
	HdrAEAD := aead.Seal(nil, key[chacha20poly1305.KeySize:chacha20poly1305.KeySize+aead.NonceSize()], data, nil)

	outputBuffer := utils.GetBuf()
	defer utils.PutBuf(outputBuffer)
	outputBuffer.Write(generatedAuthID[:])
	outputBuffer.Write(HdrLengthAEAD)
	outputBuffer.Write(HdrAEAD)

	return outputBuffer.Bytes()
}

// from v2fly/v2ray-core/proxy/vmess/aead/encrypt.go/OpenVMessAEADHeader.
// key 必须是16字节长. v2ray 的代码返回值没命名，不可取，我们加上。
func openAEADHeader(key []byte, remainDataReader io.Reader) (aeadData []byte, shouldDrain bool, bytesRead int, errorReason error) {

	var HdrLengthAEAD [2 + chacha20poly1305.Overhead]byte

	HdrLengthAEADReadBytesCounts, err := io.ReadFull(remainDataReader, HdrLengthAEAD[:])
	bytesRead += HdrLengthAEADReadBytesCounts
	if err != nil {
		return nil, false, bytesRead, err
	}

	key = kdf(key, []byte(kdfSaltConstAEADKEY), []byte("HeaderLength"))
	aead, err := chacha20poly1305.New(key[:chacha20poly1305.KeySize])
	if err != nil {
		return nil, false, bytesRead, err
	}
	HdrLength, err := aead.Open(nil, key[chacha20poly1305.KeySize:chacha20poly1305.KeySize+chacha20poly1305.NonceSize], HdrLengthAEAD[:], nil)
	if err != nil {
		return nil, false, bytesRead, err
	}
	var length uint16
	if err := binary.Read(bytes.NewReader(HdrLength), binary.BigEndian, &length); err != nil {
		panic(err)
	}

	HdrAEAD := make([]byte, length+chacha20poly1305.Overhead)

	HdrAEADReadedBytesCounts, err := io.ReadFull(remainDataReader, HdrAEAD)
	bytesRead += HdrAEADReadedBytesCounts
	if err != nil {
		return nil, false, bytesRead, err
	}

	key = kdf(key, []byte(kdfSaltConstAEADKEY), []byte("Header"))
	aead, err = chacha20poly1305.New(key[:chacha20poly1305.KeySize])
	if err != nil {
		return nil, false, bytesRead, err
	}
	decryptedHdr, err := aead.Open(nil, key[chacha20poly1305.KeySize:chacha20poly1305.KeySize+chacha20poly1305.NonceSize], HdrAEAD, nil)
	if err != nil {
		return nil, true, bytesRead, err
	}

	return decryptedHdr, false, bytesRead, nil
}
