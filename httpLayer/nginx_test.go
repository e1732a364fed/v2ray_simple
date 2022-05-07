package httpLayer_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
)

func TestNginxResponse(t *testing.T) {
	t.Log(httpLayer.GetReal400Response())
}
