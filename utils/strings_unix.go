//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package utils

import (
	"github.com/tjarratt/babble"
	"go.uber.org/zap"
)

func GetRandomWord() (result string) {
	//babbler包 在 系统中 没有 /usr/share/dict/words 且不是windows 时，会panic
	defer func() {

		if r := recover(); r != nil {
			if ce := CanLogErr("getRandomWord babble panic"); ce != nil {
				ce.Write(zap.Any("err:", r))
			}

			result = GenerateRandomString()
		}
	}()
	babbler := babble.NewBabbler()
	babbler.Count = 1
	result = babbler.Babble()

	return
}
