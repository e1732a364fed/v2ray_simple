package tlsLayer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func copyTls12Handshake(isSrcClient bool, dst io.Writer, src io.Reader) error {
	var tls_plaintxt [5]byte
	step := 0
	thisChangeCipher := false
	lastHandshakeTogo := false
	for {
		if ce := utils.CanLogDebug("shadowTls copy "); ce != nil {
			ce.Write(zap.Int("step", step),
				zap.Bool("isSrcClient", isSrcClient),
			)
		}

		_, err := io.ReadFull(src, tls_plaintxt[:])
		if err != nil {
			return err
		}
		contentType := tls_plaintxt[0]

		if ce := utils.CanLogDebug("shadowTls copy "); ce != nil {
			ce.Write(zap.Int("step", step),
				zap.Bool("isSrcClient", isSrcClient),
				zap.Uint8("contentType", contentType),
			)
		}

		if contentType != 22 {

			if step == 0 {
				return errors.New("copyUntilHandshakeEnd: tls_plaintxt[0]!=22")
			} else {

				if contentType == 20 {
					thisChangeCipher = true
				} else {
					return utils.ErrInErr{ErrDesc: "copyUntilHandshakeEnd: contentType wrong, mustbe 20 or 22", Data: contentType}
				}
				//}

			}
		}

		length := binary.BigEndian.Uint16(tls_plaintxt[3:])
		_, err = io.Copy(dst, io.MultiReader(bytes.NewReader(tls_plaintxt[:]), io.LimitReader(src, int64(length))))
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
	}
	return nil
}
