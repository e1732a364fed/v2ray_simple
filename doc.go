/*
Package v2ray_simple provides a simple way to set up a proxy.

Structure 本项目结构

utils -> netLayer-> tlsLayer -> httpLayer -> advLayer -> proxy -> v2ray_simple -> cmd/verysimple

根项目 v2ray_simple 仅研究实际转发过程. 关于 代理的详细定义 请参考 proxy 子包的文档。

本项目是一个代理项目，最重要的事情就是 【如何转发流量】，所以主项目主要研究这个转发过程。

目前主要内容有：

ListenSer函数用于主要 代理的转发，ListenTproxy函数用于转发透明代理。内置了 lazy的转发逻辑。

Tproxy

tproxy没有 TLS层 以及更上层的所有参数，只包含网络层和传输层的参数，网络层只包含 ip和端口这两个参数，传输层只包含 "是否开启sniffing" 这一个参数; 然后还有一个 tag参数，然后就没了。

所以tproxy不能算是一个完整的 proxy.Server. 所以要用单独的函数启动转发。


Chain

具体 转发过程 的 调用链 是 ListenSer -> handleNewIncomeConnection ->
handshakeInserver_and_passToOutClient -> { handshakeInserver , passToOutClient ->
[ ( checkfallback) -> dialClient_andRelay -> 「 dialClient ( -> dialInnerProxy ),
netLayer.Relay / netLayer.RelayUDP 」 ] }

用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过http头、高级层、innerMux, 都搞好后，进行 proxy 握手，然后就开始转发。

因为tproxy不满足 proxy.Server 接口，所以我们 还单独提供一个 ListenTproxy 函数。

TLS Lazy Encryption - Lazy

TLS Lazy Encryption 技术 可简称为 tls lazy encrypt, tls lazy 或者 lazy.

lazy 是一种 独特的 转发方式，所以也是在 本包中处理，而不是在proxy包中处理。
proxy包只负责定义，不负责实际转发。

lazy 与 xtls类似，在一定条件下可以利用内层tls加密直接传输数据，不在外面再包一层tls。

lazy与xtls的不同是，lazy不魔改tls包，所以是可以在 uTLS 的基础上 进行 lazy的，而且也没有 xtls的233漏洞。

目前lazy还在完善阶段。


Tags

本包提供 noquic, grpc_full 这两个 build tag。

若 grpc_full 给出，则引用 advLayer/grpc 包，否则默认引用 advLayer/grpcSimple 包。
grpcSimple 比 grpc 节省 大概 4MB 大小。

若 noquic给出，则不引用 advLayer/quic，否则 默认引用 advLayer/quic。
quic大概占用 2MB 大小。

*/
package v2ray_simple
