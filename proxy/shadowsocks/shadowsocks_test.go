package shadowsocks_test

import (
	"testing"
)

func TestTCP(t *testing.T) {
	//proxy.TestTCP("shadowsocks", "aes-128-gcm:iloveverysimple", 0, netLayer.RandPortStr_safe(true, false), "", t)

	//无法在不报错的情况下用当前代码同时在一个程序中建立服务端和客户端，
	// 因为这会导致client刚使用一个salt进行Write后，就会把该salt保存到bloom过滤器中
	//server因为使用同样的internal代码，读取它的时候，就会检测出这个salt已经用过，导致报错
	// 错误内容为 repeated salt detected

	//这里把该情况记录下来。 测试只能使用外部测试，ss已经通过测试。
}
