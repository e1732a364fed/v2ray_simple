package http

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
)

const tempClientConfStr = `
[[listen]]
protocol = "http"
`

func SetupTmpProxyServer() (clientEndInServer proxy.Server, proxyUrl string, err error) {

	var clientConf proxy.StandardConf

	clientConf, err = proxy.LoadTomlConfStr(tempClientConfStr)
	if err != nil {
		return
	}

	clientEndInServer, err = proxy.NewServer(clientConf.Listen[0])
	if err != nil {
		return
	}
	listenAddrStr := netLayer.GetRandLocalPrivateAddr(true, false)
	clientEndInServer.SetAddrStr(listenAddrStr)

	proxyUrl = "http://" + listenAddrStr

	return
}
