//go:build android && androidAAR

package machine

//安卓我们直接将proxy等子包引入. 这是为了方便直接用machine包编译aar
import (
	_ "github.com/e1732a364fed/v2ray_simple/advLayer/grpcSimple"
	_ "github.com/e1732a364fed/v2ray_simple/advLayer/quic"
	_ "github.com/e1732a364fed/v2ray_simple/advLayer/ws"
	"github.com/e1732a364fed/v2ray_simple/utils"

	_ "github.com/e1732a364fed/v2ray_simple/proxy/dokodemo"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/shadowsocks"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/simplesocks"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/socks5http"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/tproxy"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/trojan"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/tun"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vless"
	_ "github.com/e1732a364fed/v2ray_simple/proxy/vmess"
)

func init() {
	utils.LogLevel = utils.Log_warning
	utils.InitLog("init android log")
}
