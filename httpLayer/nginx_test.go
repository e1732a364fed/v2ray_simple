package httpLayer_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
)

func TestNginxResponse(t *testing.T) {
	t.Log(httpLayer.GetNginx400Response())
	if len(httpLayer.Nginx403_html) != 169 {
		t.Log("len(httpLayer.Nginx403_html)!=169", len(httpLayer.Nginx403_html))
		t.FailNow()
	}
}
