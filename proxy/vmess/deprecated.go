package vmess

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

//Deprecated: send non_aead_auth info: HMAC("md5", UUID, UTC)
func (c *ClientConn) non_aead_auth() error {
	ts := utils.GetBytes(8)
	defer utils.PutBytes(ts)

	binary.BigEndian.PutUint64(ts, uint64(time.Now().UTC().Unix()))

	h := hmac.New(md5.New, c.user.IdentityBytes())
	h.Write(ts)

	_, err := c.Conn.Write(h.Sum(nil))
	return err
}

//Deprecated: non_aead is depreated
func (c *ClientConn) non_aead_decodeRespHeader() error {
	block, err := aes.NewCipher(c.respBodyKey[:])
	if err != nil {
		return err
	}

	stream := cipher.NewCFBDecrypter(block, c.respBodyIV[:])

	b := utils.GetBytes(4)
	defer utils.PutBytes(b)

	_, err = io.ReadFull(c.Conn, b)
	if err != nil {
		return err
	}

	stream.XORKeyStream(b, b)

	if b[0] != c.reqRespV {
		return errors.New("unexpected response header")
	}

	if b[2] != 0 {
		return errors.New("dynamic port is not supported now")
	}

	return nil
}
