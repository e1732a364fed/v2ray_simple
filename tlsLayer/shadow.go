package tlsLayer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

func shadowTls1(servername string, clientConn net.Conn) (tlsConn *Conn, err error) {
	var fakeConn net.Conn
	fakeConn, err = net.Dial("tcp", servername+":443")
	if err != nil {
		if ce := utils.CanLogErr("Failed shadowTls server fake dial server "); ce != nil {
			ce.Write(zap.Error(err))
		}
		return
	}
	if ce := utils.CanLogDebug("shadowTls ready to fake "); ce != nil {
		ce.Write()
	}

	var wg sync.WaitGroup
	var e1, e2 error
	wg.Add(2)
	go func() {
		e1 = copyTls12Handshake(true, fakeConn, clientConn)
		wg.Done()

		if ce := utils.CanLogDebug("shadowTls copy client end"); ce != nil {
			ce.Write(zap.Error(e1))
		}
	}()
	go func() {
		e2 = copyTls12Handshake(false, clientConn, fakeConn)
		wg.Done()

		if ce := utils.CanLogDebug("shadowTls copy server end"); ce != nil {
			ce.Write(
				zap.Error(e2),
			)
		}
	}()

	wg.Wait()

	if e1 != nil || e2 != nil {
		e := utils.Errs{}
		e.Add(utils.ErrsItem{Index: 1, E: e1})
		e.Add(utils.ErrsItem{Index: 2, E: e2})
		return nil, e
	}

	if ce := utils.CanLogDebug("shadowTls fake ok "); ce != nil {
		ce.Write()
	}

	tlsConn = &Conn{
		Conn: clientConn,
	}

	return
}

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
