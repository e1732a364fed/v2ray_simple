package http

import (
	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

func SetupTmpProxyServer() (clientEndInServer proxy.Server, proxyUrl string, err error) {
	const tempClientConfStr = `
	protocol = "http"
	`

	var lc proxy.ListenConf
	_, err = toml.Decode(tempClientConfStr, &lc)
	if err != nil {
		return
	}

	clientEndInServer, err = proxy.NewServer(&lc)
	if err != nil {
		return
	}
	listenAddrStr := netLayer.GetRandLocalPrivateAddr(true, false)
	clientEndInServer.SetAddrStr(listenAddrStr)

	proxyUrl = "http://" + listenAddrStr

	return
}
