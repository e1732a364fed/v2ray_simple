/*
Package v2ray_simple provides a simple way to set up a proxy.

Structure 本项目结构

utils -> netLayer-> tlsLayer -> httpLayer -> advLayer -> proxy -> v2ray_simple -> cmd/verysimple

根项目 v2ray_simple 仅研究实际转发过程. 关于 代理的详细定义 请参考 proxy 子包的文档。

Chain

具体 转发过程 的 调用链 是 ListenSer -> handleNewIncomeConnection ->
handshakeInserver_and_passToOutClient -> { handshakeInserver , passToOutClient ->
[ ( checkfallback) -> dialClient_andRelay -> 「 dialClient ( -> dialInnerProxy ),
netLayer.Relay / netLayer.RelayUDP 」 ] }

用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过http头、高级层、innerMux, 都搞好后，进行 proxy 握手，然后就开始转发

因为tproxy不满足 proxy.Client 接口，所以我们 还单独提供一个 ListenTproxy 函数。

Tags

本包提供 noquic, grpc_full 这两个 build tag。

若 grpc_full 给出，则引用 advLayer/grpc 包，否则默认引用 advLayer/grpcSimple 包。
grpcSimple 比 grpc 节省 大概 4MB 大小。

若 noquic给出，则不引用 advLayer/quic，否则 默认引用 advLayer/quic。
quic大概占用 2MB 大小。

*/
package v2ray_simple
