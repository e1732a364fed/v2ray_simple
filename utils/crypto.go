package utils

import (
	"crypto/hmac"
	"crypto/sha1"
	"hash"
	"io"
	"runtime"
)

// 有些系统对aes支持不好，有些支持好。SystemAutoUseAes若为true，则说明支持很好，使用aes作为加密算法速度最佳。
const SystemAutoUseAes = runtime.GOARCH == "amd64" || runtime.GOARCH == "s390x" || runtime.GOARCH == "arm64"

type HashReader struct {
	io.Reader
	hmac hash.Hash
}

func NewHashReader(conn io.Reader, key []byte) *HashReader {
	return &HashReader{
		conn,
		hmac.New(sha1.New, key),
	}
}

func (c *HashReader) Read(b []byte) (n int, err error) {
	n, err = c.Reader.Read(b)
	if err != nil {
		return
	}
	_, err = c.hmac.Write(b[:n])
	return
}

func (c *HashReader) Sum() []byte {
	return c.hmac.Sum(nil)[:8]
}

type HashWriter struct {
	io.Writer
	hmac    hash.Hash
	written bool
}

func NewHashWriter(conn io.Writer, key []byte) *HashWriter {
	return &HashWriter{
		Writer: conn,
		hmac:   hmac.New(sha1.New, key),
	}
}

func (c *HashWriter) Write(p []byte) (n int, err error) {
	if c.hmac != nil {

		c.hmac.Write(p)
		c.written = true
	}
	return c.Writer.Write(p)
}

func (c *HashWriter) Sum() []byte {
	return c.hmac.Sum(nil)[:8]
}

func (c *HashWriter) StopHashing() {
	c.hmac = nil
	c.written = false
}

// Has the hash been written
func (c *HashWriter) Written() bool {
	return c.written
}
