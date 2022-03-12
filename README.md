# V2ray Simple （verysimple)

V2ray Simple,  建议读作 very simple (显然只适用于汉语母语者), 

正式项目名称是v2ray simple（大小写、带不带连词符或下划线均可），平时可以直接用 very simple 或 verysimple 指代。直接在任何场合 用verysimple 这个名称都是可以的，但是项目名字要弄清楚，是 v2ray_simple

verysimple项目大大简化了 转发机制，能提高运行速度。

## 特点

实现了vless协议（v0，v1）和vlesss（即vless+tcp+tls），入口使用socks5协议

在本项目里 制定 并实现了 vless v1标准。

### 关于vless v1

这里的v1是我自己制定的，总是要摸着石头过河嘛。标准的讨论详见 [vless_v1](vless_v1.md)

在客户端的 配置url中，添加 `?version=1` 即可生效。

我 实现了 一种独创的 非mux型“隔离信道”方法的 udp over tcp 的fullcone

测试 fullcone 的话，由于目前 verysimple 客户端只支持socks5入口，可以考虑先用v2ray + Netch或者透明代理 等方法监听本地网卡的所有请求，发送到 verysimple 客户端的socks5端口，然后 verysimple 客户端 再用 vless v1 发送到 v2simple vless v1 + direct 的服务端。



### 关于udp

本项目 vless 和 socks5 均支持 udp

最新的代码已经完整支持vless v0

后来我还自己实现了vless v1，自然也是支持udp的，也支持fullcone。v1还处于测试阶段

### tls lazy encrypt (splice) 

在最新代码里，还实现了 双向 tls lazy encrypt, 即另一种 xtls的 splice的实现，底层也是会调用splice，本包为了加以区分，就把这种方式叫做 tls lazy encrypt。

tls lazy encrypt 特性 运行时可以用 -lazy 参数打开（服务端客户端都要打开），然后可以用 -pdd 参数 打印 tls 探测输出

因为是双向的，而xtls的splice是单向，所以 理论上 tls lazy encrypt 比xtls 还快，应该是正好快一倍？不懂。反正我是读写都是用的splice。

而且这种技术不通过魔改tls包实现，而是在tls的外部实现，不会有我讲的xtls的233漏洞，而且以后可以与utls配合 进行模拟指纹。

关于 splice，还可以参考我的文章 https://github.com/hahahrfool/xray_splice-

该特性不稳定，会导致一些网页访问有时出现异常

不是速度慢，应该是因为 目前的tls过滤方式有点问题, 对close_alert等情况没处理好。而且使用不同的浏览器，现象也会不同，似乎对safari支持好一些， chrome就差很多

在我的最新代码里，采用了独特的技术，已经规避了大部分不稳定性。有时网页显示可能会出点问题，但是刷新网页一般可以解决；总之比较适合看视频，毕竟双向splice，不是白给的！

经过我后来的思考，发现似乎xtls的splice之所以是单向的，就是因为它在Write时需要过滤掉一些 alert的情况，否则容易被探测；

不过根据 [a report by gfwrev](https://twitter.com/gfwrev/status/1327670741597179906), 对拷直连 还是会有很多问题，很难解决

所以既然问题无法解决，不如直接应用双向splice，也不用过滤任何alert问题。破罐子破摔。

总之这种splice东西只适用于玩一玩，xtls以及所有类似的 对拷直连的 技术都是不可靠的。我只是放这里练一下手。大家玩一玩就行。

我只是在内网自己试试玩一玩，从来不会真正用于安全性要求高的用途。

注意，因为技术实现不同，该功能不兼容xtls。

### ws/grpc

以后会添加ws/grpc的支持。并且对于ws/grpc，我设计的vless v1协议会针对它们 有专门的udp优化。
## 安装方式：

```go
git clone https://github.com/hahahrfool/v2ray_simple
cd v2ray_simple && go build
cp client.example.json client.json
cp server.example.json server.json
```

## 使用方式

```sh
#客户端
v2ray_simple -c client.json

#服务端
v2ray_simple -c server.json
```

关于 vlesss 的配置，查看 server.example.json和 client.example.json就知道了，很简单的。

## 验证方式

对于功能的golang test，请使用 `go test ./...` 命令。如果要详细的打印出test的过程，可以添加 -v 参数


## 开发标准以及理念

文档尽量多，代码尽量少
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

这样也杜绝了 原作者跑路 导致的 一些不懂法律的人对于开源许可的 质疑。

实际上是毫无问题的，关键是他们太谨慎。无所谓，现在我完全自己写，没话说了吧—；

我fork也是尊重原作者，既然你们这么谨慎，正好推动了我的重构计划，推动了历史发展
## 额外说明

verysimple 是一个很简单的项目，覆盖协议也没有v2ray全，比如socks协议只能用于客户端入口，没法用于出口。

本项目的目的类似于一种 proof of concept. 方便理解，也因为极简所以能比官方v2ray快一些。

也因为是poc，所以我有时会尝试向 verysimple 中添加一些我设计的新功能。目前正在计划的有

1. 完善并实现 vless v1协议
2. 什么时候搞一个 verysimple_c 项目，用c语言照着写一遍; 也就是说，就算本verysimple没有任何技术创新，单单架构简单也是有技术优势的，可以作为参考 实现更底层的 c语言实现。
3. verysimple_c 写好后，就可以尝试将 naiveproxy 嵌入 verysimple_c 了

verysimple 继承 v2simple的一个优点，就是服务端的配置也可以用url做到。谁规定url只能用于分享客户端配置了？一条url肯定比json更容易配置，不容易出错。

不过，显然url无法配置大量复杂的内容，而且有些玩家也喜欢一份配置可以搞定多种内核，所以未来 verysimple 会推出兼容 v2ray的json配置 的模块。



## 交叉编译

版本号自己修改下即可

```sh
GOARCH=amd64 GOOS=linux go build  -trimpath -ldflags "-s -w -buildid="  -o v2ray_simple_linux_amd64_v1.0.0
GOARCH=arm64 GOOS=linux go build  -trimpath -ldflags "-s -w -buildid="  -o v2ray_simple_linux_arm64_v1.0.0
GOARCH=amd64 GOOS=windows go build  -trimpath -ldflags "-s -w -buildid="  -o v2ray_simple_win10_v1.0.0.exe
```

## 交流

https://t.me/shadowrocket_unofficial
