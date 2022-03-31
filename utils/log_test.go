package utils

import (
	"errors"
	"testing"

	"go.uber.org/zap"
)

func TestZaplog(t *testing.T) {

	LogLevel = Log_info
	InitLog()

	if ce := CanLogDebug("test1"); ce != nil {
		ce.Write(
			zap.Uint32("uid", 32),
			zap.Error(errors.New("asdfdsf")),
		)
	}

	if ce := CanLogInfo("test2"); ce != nil {
		ce.Write(
			zap.Uint32("uid", 32),
			zap.Error(errors.New("asdfdsf")),
		)
	}
}
