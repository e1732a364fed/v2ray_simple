# V2ray Simple （verysimple)

V2ray Simple,  建议读作 very simple (显然只适用于汉语母语者), 

正式名称是v2ray simple（大小写、带不带连词符或下划线均可），平时可以直接用 very simple 或 verysimple 指代。

实现了vless协议（v0，v1）和vlesss（即vless+tcp+tls）。

在本项目里 制定 并实现了 vless v1标准。


安装方式：

```go
git clone https://github.com/hahahrfool/v2ray_simple
cd v2ray_simple && go build
```

使用方式

```
#客户端
v2ray_simple -c client.json

#服务端
v2ray_simple -c server.json
```

关于 vlesss 的配置，查看我新添加的 server.json和 client.json就知道了，很简单的。

## 开发标准以及理念

### 文档

文档、注释尽量详细，且尽量完全使用中文，尽量符合golang的各种推荐标准。

根据golang的标准，注释就是文档本身（godoc的原理），所以一定要多写注释。

再次重复，文档越多越好，尽量降低开发者入门的门槛。

### 代码

代码的理念就是极简！这也是本项目名字由来！

根据 奥卡姆剃刀原理，不要搞一大堆复杂机制，最简单的能实现的代码就是最好的代码。


## 本项目所使用的开源协议

MIT协议，即你用的时候也要附带一个MIT文件，然后我不承担任何后果。

## 历史

启发自我fork的v2simple，不过原作者的架构还是有点欠缺，我就直接完全重构了，完全使用我自己的代码。

这样也杜绝了 原作者跑路 导致的 一些不懂法律的人对于开源许可的 质疑。（实际上是毫无问题的，关键是他们太谨慎。无所谓，现在我完全自己写，你们没话说了吧—；我fork也是尊重原作者，既然你们这么谨慎，正好推动了我的重构计划，推动了历史发展）
## 额外说明

verysimple 是一个很简单的项目，覆盖协议也没有v2ray全，比如socks协议只能用于客户端入口，没法用于出口。

本项目的目的类似于一种 proof of concept. 方便理解，也因为极简所以能比官方v2ray快一些。

也因为是poc，所以我有时会尝试向 verysimple 中添加一些我设计的新功能。目前正在计划的有

1. 实现 vless v1协议
2. 什么时候搞一个 verysimple_c 项目，用c语言照着写一遍
3. verysimple_c 写好后，就可以尝试将 naiveproxy 嵌入 verysimple_c 了

## 关于vless v1

这里的v1是我自己制定的，总是要摸着石头过河嘛。标准的讨论详见 [vless_v1](vless_v1.md)

在客户端的 配置url中，添加 `?version=1` 即可生效。

我 实现了 一种独创的 非mux型“隔离信道”方法的 udp over tcp 的fullcone

测试的话，由于目前 verysimple 客户端只支持socks5入口，可以考虑先用v2ray + Netch或者透明代理 等方法监听本地网卡的所有请求，发送到 verysimple 客户端的socks5端口，然后 verysimple 客户端 再用 vless v1 发送到 v2simple vless v1 + direct 的服务端。



## 关于udp

本项目 vless 和 socks5 均支持 udp

不过，我的vless支持的这个udp 暂时不符合 rprx的 v0标准，因为rprx硬是要加数据长度头，
实际上根据我的探讨，这是不必要的，因为有tls，具体可以查看我的探讨 https://github.com/v2fly/v2ray-core/discussions/1655 ， 以及 vless_m1.md

后来我还自己实现了vless v1，自然也是支持udp的，也支持fullcone。

因此如果传输udp数据的话，目前 v2simple 的vless v0 是不兼容 v2ray官方的 vless v0 的。如果只是传输纯tcp的话则完全兼容。（单纯做网页代理是不影响的，因为网页统统使用tcp；游戏和视频可能影响，因此 **此时** 用 verysimple 的话最好服务端客户端都用v2simple）


## 关于验证

对于功能的golang test，请使用 `go test ./... -v` 命令。


## 腾讯视频问题
如果用此代理打开腾讯视频网页的话，会发现视频加载不出来。bilibili是没问题的。而且按理说和udp没关系因为websocket还是基于tcp的。

经过我测试， verysimple 返回的错误是:
```
 failed to dail 'apd-9d8f8b192cbf63303ebad8d58b51293f.v.sm:443': dial tcp: lookup apd-9d8f8b192cbf63303ebad8d58b51293f.v.sm: no such host
```

此问题有待考证解决。也不知道是不是只有我自己有这个问题。。


## 交流

https://t.me/shadowrocket_unofficial
