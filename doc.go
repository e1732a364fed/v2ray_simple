/*
Package v2ray_simple provides a way to set up a proxy.


Structure 本项目结构

utils -> netLayer-> tlsLayer -> httpLayer -> advLayer -> proxy -> v2ray_simple -> cmd/verysimple


Chain

具体 转发过程 的 调用链 是 ListenSer -> handleNewIncomeConnection -> handshakeInserver_and_passToOutClient -> handshakeInserver -> passToOutClient ( -> checkfallback) -> dialClient_andRelay -> dialClient ( -> dialInnerMux ),  netLayer.Relay / netLayer.RelayUDP

用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过ws/grpc, 然后都搞好后，传到proxy，然后就开始转发

当然如果遇到quic这种自己处理从传输层到高级层所有阶段的“超级协议”的话, 在操纵路由后直接传给 quic，然后quic握手建立后直接传到 proxy
*/
package v2ray_simple
