/*
Package v2ray_simple provides a way to set up a proxy.


Config Format  配置格式

一共有三种配置格式，极简模式，标准模式，兼容模式。

“极简模式”(即 verysimple mode)，入口和出口仅有一个，而且都是使用共享链接的url格式来配置.

标准模式使用toml格式。

兼容模式可以兼容v2ray现有json格式。（暂未实现）。

极简模式的理念是，配置文件的字符尽量少，尽量短小精悍;

还有个命令行模式，就是直接把极简模式的url 放到命令行参数中，比如:

	verysimple -L socks5://sfdfsaf -D direct://


Structure 本项目结构

utils -> netLayer-> tlsLayer -> httpLayer -> advLayer -> proxy -> v2ray_simple


Chain

具体 转发过程 的 调用链 是 ListenSer -> handleNewIncomeConnection -> handshakeInserver_and_passToOutClient -> handshakeInserver -> passToOutClient ( -> checkfallback) -> dialClient_andRelay -> dialClient ( -> dialInnerMux ),  netLayer.Relay / netLayer.RelayUDP

用 netLayer操纵路由，用tlsLayer嗅探tls，用httpLayer操纵回落，可选经过ws/grpc, 然后都搞好后，传到proxy，然后就开始转发

当然如果遇到quic这种自己处理从传输层到高级层所有阶段的“超级协议”的话, 在操纵路由后直接传给 quic，然后quic握手建立后直接传到 proxy
*/
package v2ray_simple
