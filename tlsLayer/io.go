package tlsLayer

import (
	"bytes"
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
