package tlsLayer

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

// isSrcClient is only for debug logging.
func CopyTls12Handshake(isSrcClient bool, dst, src net.Conn) error {
	var tls_plaintxt [5]byte
	step := 0
	thisChangeCipher := false
	lastHandshakeTogo := false
	for {
		if ce := utils.CanLogDebug("copyTls12Handshake "); ce != nil {
			ce.Write(zap.Int("step", step),
				zap.Bool("isSrcClient", isSrcClient),
			)
		}

		netLayer.SetCommonReadTimeout(src)
		_, err := io.ReadFull(src, tls_plaintxt[:])
		netLayer.PersistRead(src)

		if err != nil {
			return err
		}
		contentType := tls_plaintxt[0]

		if ce := utils.CanLogDebug("copyTls12Handshake "); ce != nil {
			ce.Write(zap.Int("step", step),
				zap.Bool("isSrcClient", isSrcClient),
				zap.Uint8("contentType", contentType),
			)
		}

		if contentType != 22 {

			if step == 0 {
				return errors.New("copyTls12Handshake: tls_plaintxt[0]!=22")
			} else {

				if contentType == 20 {
					thisChangeCipher = true
				} else {
					return utils.ErrInErr{ErrDesc: "copyTls12Handshake: contentType wrong, mustbe 20 or 22", Data: contentType}
				}

			}
		}

		length := binary.BigEndian.Uint16(tls_plaintxt[3:])

		netLayer.SetCommonReadTimeout(src)
		netLayer.SetCommonWriteTimeout(dst)

		_, err = io.Copy(dst, io.MultiReader(bytes.NewReader(tls_plaintxt[:]), io.LimitReader(src, int64(length))))

		netLayer.PersistRead(src)
		netLayer.PersistWrite(dst)

		if err != nil {
			return err
		}
		if lastHandshakeTogo {
			break
		}
		if thisChangeCipher {
			lastHandshakeTogo = true
		}

		step += 1
		if step > 6 {
			return errors.New("shit, shadowTls copy loop > 6, maybe under attack")

		}
	}
	return nil
}

func WriteAppData(conn io.Writer, buf *bytes.Buffer, d []byte) (n int, err error) {

	shouldPut := false

	if buf == nil {
		buf = utils.GetBuf()
		shouldPut = true
	}
	WriteAppDataHeader(buf, len(d))

	buf.Write(d)

	n, err = conn.Write(buf.Bytes())

	if shouldPut {
		utils.PutBuf(buf)

	}
	return
}

// 一般conn直接为tcp连接，而它是有系统缓存的，因此我们一般不需要特地创建一个缓存
// 写两遍之后在发出
func WriteAppDataNoBuf(w io.Writer, d []byte) (n int, err error) {

	err = WriteAppDataHeader(w, len(d))
	if err != nil {
		return
	}
	return w.Write(d)

}

func WriteAppDataHeader(w io.Writer, len int) (err error) {
	var h [5]byte
	h[0] = 23
	binary.BigEndian.PutUint16(h[1:3], tls.VersionTLS12)
	binary.BigEndian.PutUint16(h[3:], uint16(len))

	_, err = w.Write(h[:])
	return
}
