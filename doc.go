/*
Package v2ray_simple provides a way to set up a proxy.

Structure 本项目结构

utils -> netLayer-> tlsLayer -> httpLayer -> advLayer -> proxy -> v2ray_simple -> cmd/verysimple

根项目 v2ray_simple 仅研究实际转发过程.

Chain

具体 转发过程 的 调用链 是 ListenSer -> handleNewIncomeConnection ->
handshakeInserver_and_passToOutClient -> { handshakeInserver , passToOutClient ->
[ ( checkfallback) -> dialClient_andRelay -> 「 dialClient ( -> dialInnerProxy ),
netLayer.Relay / netLayer.RelayUDP 」 ] }

用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过高级层, 都搞好后，传到proxy，然后就开始转发

Tags

本包提供 noquic, grpc_full 这两个 build tag。

若 grpc_full 给出，则引用 advLayer/grpc 包，否则默认引用 advLayer/grpcSimple 包。
若 noquic给出，则不引用 advLayer/quic，否则 默认引用 advLayer/quic。


*/
package v2ray_simple
