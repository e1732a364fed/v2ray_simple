![GoVersion][10] [![GoDoc][1]][2] [![MIT licensed][3]][4] [![Go Report Card][5]][6] [![Downloads][7]][8] [![release][9]][8] 

[1]: https://pkg.go.dev/badge/github.com/e1732a364fed/v2ray_simple.svg
[2]: https://pkg.go.dev/github.com/e1732a364fed/v2ray_simple#section-readme
[3]: https://img.shields.io/badge/license-MIT-blue.svg
[4]: LICENSE
[5]: https://goreportcard.com/badge/github.com/e1732a364fed/v2ray_simple
[6]: https://goreportcard.com/report/github.com/e1732a364fed/v2ray_simple
[7]: https://img.shields.io/github/downloads/e1732a364fed/v2ray_simple/total.svg
[8]: https://github.com/e1732a364fed/v2ray_simple/releases/latest
[9]: https://img.shields.io/github/release/e1732a364fed/v2ray_simple/all.svg?style=flat-square
[10]: https://img.shields.io/github/go-mod/go-version/e1732a364fed/v2ray_simple?style=flat-square

### 最新消息

最近在开发vsb计划，即verysimple 面板项目，在 https://github.com/e1732a364fed/vsb , 可以看看。

# verysimple

verysimple， 实际上 谐音来自 V2ray Simple (显然只适用于汉语母语者), 意思就是极简.

verysimple 是一个 代理内核, 对标 v2ray/xray，功能较为丰富，轻量级，极简，用户友好，新手向。

本作的想法是，使用自己的代码，实现v2ray的所有的好的功能（并摒弃差的功能），而且使用自主研发的更简单的架构，结合自主研发的新技术，实现反超。

verysimple项目大大简化了 转发机制，能提高运行速度。本项目 转发流量时，关键代码直接放在main.go里！非常直白易懂。

只有项目名称是v2ray_simple，其它所有场合 全使用 verysimple 这个名称，可简称 "vs"。本作过于极简，极简得连logo也没有.

规定，编译出的文件名必须以 verysimple 开头.

verysimple 研发了一些新技术，使用自研架构，可以加速，目前基本上是全网最快，且有用户报告内存占用 比v2ray/xray 小1/3。

vs的一些亮点是 全协议readv加速，lazy技术，vless v1，hysteria 阻控，更广泛的utls支持，grpc回落，交互模式等。
## 支持的功能

socks5(包括 udp associate 以及用户密码)/http(以及用户密码)/socks5http(与clash的mixed等价)/dokodemo/tproxy(透明代理)/trojan/simplesocks/vless(v0/**v1**)/vmess/shadowsocks, 多用户, http头

ws(以及earlydata)/grpc(以及multiMode,uTls，以及 **支持回落的 grpcSimple**)/quic(以及**hy阻控、手动挡** 和 0-rtt)/smux, 

dns(udp/tls)/route(geoip/geosite,分流功能完全与v2ray等价)/fallback(path/sni/alpn/PROXY protocol v1/v2), sniffing(tls)

tcp/udp(以及fullcone)/unix domain socket, tls(包括客户端证书验证), uTls,**【tls lazy encrypt】**, http伪装头（**可支持回落**）,PROXY protocol v1/v2 监听,

cli(**交互模式**)/apiServer, Docker, docker-compose.


为了不吓跑小白，本 README 把安装、使用方式 放在了前面，如果你要直接阅读本作的技术介绍部分，点击跳转 -> [创新点](#创新点)



## 安装方式：

### 下载安装

如果是 linux服务器，可参考指导文章 [install.md](docs/install.md).

电脑客户端的话直接自己到release下载就行。

#### 客户端的 geoip和 geosite

注意如果要geoip分流，而且要自己的mmdb文件的话（高玩情况），还要下载mmdb；


默认 如果加上 -d 参数的话，是会自动下载mmdb文件以及 geosite 文件夹的，而且如果你指定了配置文件并且你的节点可用，则还会 自动通过 你的节点 下载 这两个文件，所以不怕被 封锁。

如果不用 -d 参数，可以通过交互模式进行下载，或通过如下命令下载

```sh
#在verysimple可执行文件所在目录
git clone https://github.com/v2fly/domain-list-community
mv domain-list-community geosite
```

通过git下载的好处是, 自己想要更新时，直接 `git pull` 即可;

通过 交互模式 或者 -d 参数进行下载的好处是, 如果你配置了配置文件, 并且有一个可用的节点, 则优先通过你的节点来下载geosite.


### 编译安装

```sh
git clone https://github.com/e1732a364fed/v2ray_simple
cd v2ray_simple/cmd/verysimple && go build
```

详细优化的编译参数请参考Makefile文件

如果你是直接下载的可执行文件，则不需要 go build

注意，本项目自v1.1.9开始，可执行文件的目录在 cmd/verysimple 文件夹内，而根目录 为 v2ray_simple 包。

## 运行方式

本作支持多种运行模式，方便不同需求的同学使用

1. 命令行模式
2. 极简模式
3. 标准模式
4. 兼容模式
5. 交互模式


### 运行前的准备

若为客户端，可运行 `./verysimple -i` 进入交互模式，选择下载geosite文件夹 以及 geoip文件(GeoLite2-Country.mmdb)

可以通过 [交互模式](#交互模式) 来生成自定义的配置。

### 极简模式

```sh
#客户端, 极简模式
verysimple -c client.json

#服务端, 极简模式
verysimple -c server.json
```

极简模式使用json格式，内部使用链接url的方式，所以非常节省空间;

关于url格式的具体写法，见： [url标准定义](docs/url.md)，我们定义了一种通用url格式。

查看 `examples/vs.server.json` 和 `examples/vs.client.json` 就知道了，很简单的。

目前极简模式配置文件最短情况一共就4行，其中两行还是花括号

极简模式 不支持 复杂分流，dns 等高级特性。极简模式只支持通过 mycountry进行 geoip分流 这一种分流情况。

极简模式继承自 v2simple, 理念是字越少越好。推荐没有极简需求的同学直接使用标准模式。

另外，verysimple 继承 v2simple的一个优点，就是服务端的配置也可以用url做到。谁规定url只能用于分享客户端配置了？一条url肯定比json更容易配置，不容易出错。


### 命令行模式

学会了极简模式里的url配置后，还可以用如下命令来运行，无需配置文件

```sh
#客户端
verysimple -L=socks5://127.0.0.1:10800 -D=vlesss://你的uuid@你的服务器ip:443?insecure=true

#服务端
verysimple -L=vlesss://你的uuid@你的服务器ip:443?cert=cert.pem&key=cert.key&version=0&fallback=:80
```

不细心的人要注意了，vlesss，要三个s，不然的话你就是裸奔状态,加了第三个s才表示套tls

命令行模式 实际上就是把命令行的内容转化成极简模式的配置 然后再处理

命令行模式 不支持dns、分流、复杂回落 等特性。只能在url中配置 默认回落。

### 标准模式

```sh
#客户端，标准模式
verysimple -c client.toml
#服务端，标准模式
verysimple -c server.toml

```

标准模式使用toml格式，类似windows的ini，对新手友好，不容易写错。推荐直接使用标准模式。

**examples文件夹中的 vlesss.client.toml, vlesss.server.toml , multi.client.toml 等文件中 提供了大量解释性的注释, 对新手很友好, 一定要读一下，才可以熟练掌握配置格式。**

我们大部分的toml示例文件都有一定的教学意义，希望用户能都读一下。

### 兼容模式

未来会推出兼容v2ray的json配置文件的模式。

### 交互模式

交互模式可以在命令行交互着生成一个你想要的配置，这样也就不需要各种一键脚本了

交互模式有很多好玩的功能，可以试试，使用起来很灵活。

运行 `verysimple -i` 即可进入交互模式; 你也可以在-c指定了配置文件的同时 指定-i，这样就可以边运行边动态调整了。

目前支持如下功能：

1. 生成随机ssl证书
2. 【交互生成配置】，超级强大
3. 【生成分享链接】<-当前的配置
4. 热删除配置
5. 【热加载】新配置文件
6. 【热加载】新配置url
7. 调节日志等级
8. 调节hy手动挡
9. 生成一个随机的uuid供你参考
10. 下载geosite文件夹
11. 下载geoip文件(GeoLite2-Country.mmdb)
12. 打印当前版本所支持的所有协议
13. 查询当前状态
14. 为tproxy设置iptables(12345端口)
15. 为tproxy移除iptables


交互生成配置后还可以输出到文件、加载到当前运行环境、生成分享链接。

### 其他说明

如果你不是放在path里的，则要 `./verysimple`, 前面要加一个点和一个斜杠。windows没这个要求。

## 关于证书

<details>

自己生成证书！而且最好是用 自己真实拥有的域名，使用acme.sh等脚本申请免费证书，特别是建站等情况。

而且用了真证书后，别忘了把配置文件中的 `insecure=true` 给删掉.

使用自签名证书是会被中间人攻击的，再次特地提醒。如果被中间人攻击，就能直接获取你的uuid，然后你的服务器 攻击者就也能用了。

要想申请真实证书，仅有ip是不够的，要拥有一个域名。本项目提供的 生成随机证书功能 仅供快速测试使用，切勿用于实际场合。

### shell 命令 生成自签名证书

注意运行第二行命令时会要求你输入一些信息。确保至少有一行不是空白即可，比如打个1
```sh
openssl ecparam -genkey -name prime256v1 -out cert.key
openssl req -new -x509 -days 7305 -key cert.key -out cert.pem
```

此命令会生成ecc证书，这个证书比rsa证书 速度更快, 有利于网速加速（加速tls握手）。

#### 使用客户端证书的 高玩情况：

小白请无视这一段。

```sh
# 生成ca的命令:
openssl ecparam -genkey -name prime256v1 -out ca.key    
openssl req -new -x509 -days 365 -sha256 -key ca.key -out ca.crt  #会提示让你输入 CountryName 等信息。

# 用ca生成客户端key和crt
openssl ecparam -genkey -name prime256v1 -out client.key
openssl req -new -key client.key -out client.csr   #会提示 让你输入 CountryName 等信息。
openssl x509 -req -days 365 -sha256  -in client.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out client.crt
```

之后, ca.crt 用于CA (服务端要配置这个), client.key 和 client.crt 用于 客户端证书 （客户端要配置这个）

注意 上面的openssl 生成 crt 的两个命令 要使用 -sha256参数, 因为默认的sha1已经不安全, 在go1.18中被废弃了。

### 交互模式 生成证书

本作的交互模式也有自动生成随机自签名证书功能

在你的服务端下载好程序后，运行 `verysimple -i` 开启交互模式，然后按向下箭头 找到对应选项，按回车 来自动生成tls证书。

</details>


# 技术相关

<details>

<summary>技术相关</summary>

## 创新点

本作有不少创新点，如下

### 协议

实现了vless协议（v0，v1）

在本项目里 制定 并实现了 vless v1标准 (还在继续研发新功能），添加了非mux的fullcone；

### lazy技术

本项目 发明了独特的非魔改tls包的 双向splice，本作称之为 tls lazy encrypt, 简称lazy

### grpcSimple

在clash的gun.go (MIT协议) grpc客户端 代码基础上实现了 grpcSimple, 包含完整的服务端, 遵循了极简的理念，不引用谷歌的grpc包，减小编译大小4MB，**而且支持 回落到 h2c**。


### 架构

使用了简单的架构，单单因为架构简单 就可以 提升不少性能。而且可执行文件比其他内核小不少。

本作使用了分层架构，网络层，tls层，高级层，代理层等层级互不影响。

所有传输方式均可使用utls来伪装指纹；
所有方式均可以选用 tcp、udp、unix domain socket 等 网络层，不再拘泥于原协议的网络层设计。

### 兼容性与速度
v0协议是直接兼容现有v2ray/xray的，比如可以客户端用任何现有支持vless的客户端，服务端使用verysimple

经过实际测速，就算不使用lazy encrypt等任何附加技术，verysimple作为服务端还是要比 v2ray做服务端要快。作客户端时也是成立的。最新1.10的测速似乎不lazy时即可比xray的 xtls快。（ [最新测速](docs/speed_macos_1.1.0.md) )

### 命令行

本作的命令行界面还有一种 “交互模式”，欢迎下载体验，使用 `-i` 参数打开。也欢迎提交PR 来丰富 交互模式的功能。

### 创新之外的已实现的有用特性

支持trojan协议 以及smux, 而且经过测速，比trojan-go快。（速度差距和本作的vless与v2ray的vless的差距基本一致，所以就不放出测速文件了，参考vless即可）

在没有mmdb文件时，自动下载mmdb

使用readv 进行加速

其它监听协议还支持 socks5, http, dokodemo,vmess, simplesocks, shadowsocks 等

多种配置文件格式,包括自有的 toml标准格式

默认回落，以及按 path/sni/alpn 回落

按 geoip,geosite,ip,cidr,domain,tag,network 分流，以及 按国别 顶级域名分流，用到了 mmdb和 v2fly的社区维护版域名列表

支持utls伪装tls指纹，本作的 utls 还可以在 用 websocket和grpc 时使用

支持websocket, 使用性能最高的 gobwas/ws 包，支持 early data 这种 0-rtt方式，应该是与现有xray/v2ray兼容的

支持grpc，与 xray/v2ray兼容; 还有 grpcSimple，见上文。

真实 nginx拒绝响应。

支持 quic以及hysteria 阻控，与xray/v2ray兼容（详情见wiki）,还新开发了“手动挡”模式

api服务器；tproxy 透明代理； http头(即所谓的混淆、伪装头等), 该模式下还支持回落。

本作支持 trojan-go 的 “可插拔模块”模式的。而且也可以用build tag 来开启或关闭某项功能。不过本作为了速度，耦合高一些。

本作也是支持 clash 的 "use as library" 的，而且 very simple，看godoc文档就懂了，主项目就一个主要的函数。

支持 Docker 容器, 见 #56，以及 cmd/verysimple/Dockerfile,  相关问题请找 该PR作者。

可生成各种第三方分享链接格式;

总之，可以看到，几乎在每一个技术上 本作都有一定的优化，超越其他内核，非常 Nice。

## 技术详情

本作虽然声称 v2ray_simple, 但是实际的理念 与 clash 和 trojan-go 更加靠近，我也更欣赏这两个包，而不是 v2ray。

这也是我单独写一个 v2ray_simple 的原因。 v2ray的架构实在是非常落后，无法施展拳脚，而clash 和 trojan-go 则先进很多。


目前认为只有外层为 tls 的、支持回落的 协议才是主流。

而 vmess这种 信息熵 太大 的协议已经 应该退出历史舞台，然而，最近墙的 sni 阻断行为再一次打我脸了。看来 vmess/ssr 这种完全随机的协议还是有必要继续使用。。。

总之，世界一直在变化，技术的采用 也要随机应变。


### 关于vless v1

这里的v1是 verysimple 自己制定的，总是要摸着石头过河嘛。标准的讨论详见 [vless_v1](docs/vless_v1.md)

总之，简单修订了一下协议格式，然后重点完善了fullcone。

verysimple 实现了 一种独创的 非mux型“分离信道”方法的 udp over tcp 的fullcone

v1还有很多其他新设计，比如用于 连接池和 dns等，详见 [vless_v1_discussion](docs/vless_v1_discussion.md)

vless v1协议还处在开发阶段，我随时可能新增、修改定义。

因为本作率先提出了 vless v1的开发，所以本作的版本号 也直接从 v1.0.0开始 (手动狗头～)

### 关于udp

本项目 完整支持 udp

最新的代码已经完整支持vless v0

后来我还自己实现了vless v1，自然也是支持udp的，也支持fullcone。v1还处于测试、研发阶段.

另外上面说的是承载数据支持udp；我们协议的底层传输方式也是全面支持udp的。也就是说可以用udp传输vless数据，然后vless里面还可以传输 udp的承载数据。

底层用udp传输的话，可以理解为 比 v2ray的mkcp传输方式 更低级的模式，直接用udp传输, 不加任何控制。所以可能丢包,导致速度较差 且不稳定。

### tls lazy encrypt (splice) 

**注意，因为技术实现不同，该功能不兼容xtls。**, 因为为了能够在tls包外进行过滤，我们需要做很多工作，所以技术实现与xtls是不一样的。

**lazy功能是对标xtls的，但是不兼容xtls，你用lazy的话，两端必须全用verysimple**

关于xtls，你还可以阅读我对 xtls的233漏洞的研究文章

https://github.com/e1732a364fed/xtls-


在最新代码里，实现了 双向 tls lazy encrypt, 即另一种 xtls的 splice的实现，底层也是会调用splice，本包为了加以区分，就把这种方式叫做 tls lazy encrypt。

tls lazy encrypt 特性 运行时可以用 -lazy 参数打开（服务端客户端都要打开），然后可以用 -pdd 参数 打印 tls 探测输出

在系统 不支持splice和sendfile 系统调用时，lazy特性等价于 xtls 的 direct 流控.

因为是双向的，而xtls的splice是单向，所以 理论上 tls lazy encrypt 比xtls 还快，应该是正好快一倍？不懂。反正我是读写都是用的splice。

而且这种技术不通过魔改tls包实现，而是在tls的外部实现，不会有我讲的xtls的233漏洞，而且以后可以与utls配合 进行模拟指纹。

关于 splice，还可以参考我的文章 https://github.com/e1732a364fed/xray_splice-

该特性不完全稳定，可能会导致一些网页访问有时出现异常,有时出现bad mac alert;刷新页面可以解决

不是速度慢，是因为 目前的tls过滤方式有点问题, 对close_alert等情况没处理好。而且使用不同的浏览器，现象也会不同

在我的最新代码里，采用了独特的技术，已经规避了大部分不稳定性。总之比较适合看视频，毕竟双向splice，不是白给的！

经过我后来的思考，发现似乎xtls的splice之所以是单向的，就是因为它在Write时需要过滤掉一些 alert的情况，否则容易被探测；

不过根据 [a report by gfwrev](https://twitter.com/gfwrev/status/1327670741597179906), 对拷直连 还是会有很多问题，很难解决

所以既然问题无法解决，不如直接应用双向splice，也不用过滤任何alert问题。破罐子破摔。

总之这种splice东西只适用于玩一玩，xtls以及所有类似的 对拷直连的 技术都是不可靠的。我只是放这里练一下手。大家玩一玩就行。

我只是在内网自己试试玩一玩，从来不会真正用于安全性要求高的用途。

关于splice的一个现有“降速”问题也要看看，（linux 的 forward配置问题），我们这里也是会存在的 https://github.com/XTLS/Xray-core/discussions/59



#### 总结 tls lazy encrypt (tle) 技术优点

解决了xtls以下痛点

1. 233 漏洞
2. 只有单向splice
3. 无法与fullcone配合
4. 无法与utls配合

原因：

1. tle 不使用循环进行tls过滤，而且不魔改tls包
2. tle直接开启了双向splice；xtls只能优化客户端性能，tle两端都会优化;一般而言大部分服务器都是linux的，所以这样就大大提升了所有连接的性能.
3. 因为tle的vless v1的fullcone是非mux的，分离信道，所以说是可以应用splice的（以后会添加支持，可能需要加一些代码，有待考察）
4. 因为tle不魔改tls包，所以说可以套任何tls包的，比如utls，目前已经添加了utls。所以你可以享受伪装的同时享受splice

而且alert根本不需要过滤，因为反正xtls本身过滤了还是有两个issue存在，是吧。

而且后面可以考虑，如果底层是使用的tls1.2，那么我们上层也可以用 tls1.2来握手。这个是可以做到的，因为底层的判断在客户端握手刚发生时就可以做到，而此时我们先判断，然后再发起对 服务端的连接，即可。

也有一种可能是，客户端的申请是带tls1.3的，但是目标服务器却返回的是tls1.2，这也是有可能的，比如目标服务器比较老，或特意关闭了tls1.3功能；此时我们可以考虑研发新技术来绕过，也要放到vless v1技术栈里。参见 https://github.com/e1732a364fed/v2ray_simple/discussions/2

在不使用新协议时，lazy只能通过不lazy tls1.2的方式来解决此问题, 即裸奔转发 tls1.3、加密转发 tls1.2. 

## 关于内嵌geoip 文件

默认的Makefile 或 直接 go build 是不开启内嵌功能的，需要加载外部mmdb文件，就是说你要自己去下载mmdb文件，

**不过，最新的版本会自动检测，如果你没有mmdb文件，会自动给你从cdn下载下来，所以已经很方便了，不需要自己动手.**

可以从 https://github.com/P3TERX/GeoLite.mmdb 项目，https://github.com/Loyalsoldier/geoip 项目， 或类似项目 进行下载

加载的外部文件 必须使用原始 mmdb格式。

若要内嵌编译，要用 `tar -czf GeoLite2-Country.mmdb.tgz GeoLite2-Country.mmdb` 来打包一下，将生成的tgz文件放到 netLayer文件夹中，然后再编译 ，用 `go build -tags embed_geoip` 编译

内嵌编译 所使用的 文件名 必须是 GeoLite2-Country.mmdb.tgz


因为为了减小文件体积，所以才内嵌的gzip格式，而不是内嵌原始mmdb

## 开发标准以及理念

KISS, Keep it Simple and Stupid

文档尽量多，代码尽量少. 同时本作不追求极致模块化, 可以进行适当耦合. 一切以速度、浅显易懂 优先

如果你阅读代码，你有时可能看到一些“比较脏” 的代码，如包含一些 goto 跳跃，或者步骤比较繁琐的函数。
但仔细思考比较，就会发现，这种代码的不是运行速度更快，就是更加直观易懂。

比如某些需要defer的地方，我们故意不defer，而是单独放在每个return前面。这是因为defer会降低性能。类似的地方有很多。

当然，如果美化代码利大于弊，我们肯定在后期慢慢改进。

### 文档

文档、注释尽量详细，且尽量完全使用中文，尽量符合golang的各种推荐标准。

根据golang的标准，注释就是文档本身（godoc的原理），所以一定要多写注释。不要以为解释重复了就不要写，因为要生成godoc文档，在 pkg.go.dev 上 给用户看的时候它们首先看到的是注释内容，而不是代码内容

本项目所生成的文档在 https://pkg.go.dev/github.com/e1732a364fed/v2ray_simple

再次重复，文档越多越好，尽量降低开发者入门的门槛。

我有时也会时常在 discussion里发一些研究、讨论的文章，大家也要踊跃发言
https://github.com/e1732a364fed/v2ray_simple/discussions


### 代码

代码的理念就是极简！这也是本项目名字由来！

根据 奥卡姆剃刀原理，不要搞一大堆复杂机制，最简单的能实现的代码就是最好的代码。

**想要为本作贡献的同学，要学习本作的这些理念，并能够贯彻你的代码。**

**不够极简或解释不够清晰的代码我们将会进行淘汰或修正。**

有贡献想法的同学，阅读 [CONTRIBUTING](CONTRIBUTING.md) / issue中的【开发者贡献指南】.

#### 开发者入门指导

首先学会使用verysimple，熟读本 README.md 和 examples/ 下的配置文件.

之后读 doc.go 和 cmd/verysimple/version.go 文件里的 注释，对本作结构有一个认识。然后读 proxy/doc.go 理解 VSI模型。

之后 学习 proxy.BaseInterface 接口 和其 实现 proxy.Base. 之后学习 advLayer 里的各个接口。

之后就可以在go doc中选择自己感兴趣的地方阅读了。

## 本项目所使用的开源协议

MIT协议，即你用的时候也要附带一个MIT文件，然后作者不承担任何责任、义务、后果。

## 历史

首先阅读v2simple项目，一个很好的启蒙项目：
https://github.com/jarvisgally/v2simple

读了v2simple后, 我fork了一个版本, 不过原作者没附带任何开源协议，而且原作者的架构还是有点欠缺。

后来就直接完全重构了，新建了本项目，完全使用自己的代码。没想到大有发展，功高盖主。

但是，本作继承了v2simple的精神，即尽量simple。我极力支持这种精神，也试图让这个精神在verysimple项目中处处体现。

本作继承了它的如下特点：

1. url配置的方式
2. 转发逻辑直接放在main.go 中
3. 架构简单


## 本作对其它项目的启发

优秀的东西总是会被模仿，但是有一些东西从未被超越。我们在模仿别人，别人也在模仿我们，不知不觉中共同创造了一个越来越棒的开源环境。

### v2ray项目
本作提倡对vless v1的进一步开发后， v2ray项目直接决定放弃vless 协议。（手动狗头～）

### xray项目
本作对xtls漏洞以及lazy技术的先行研究启发了xray项目，几个月后，其开发了 vision 流控。本作也算为代理界做出了一些贡献～。

不过xray的架构太复杂，很难将这个流控应用到所有协议上。而本作因为架构优良，lazy是直接可以用于没有内部加密的任何协议的，如vless,trojan,simplesocks,socks

本作实现grpc后就直接支持utls的，xray几个月后通过开发者的PR跟进。

### sing-box项目
本作对gun-lite客户端的先行研究，反推出了 gun-lite的服务端代码。 几个月后 sing-box也通过一个开发者的PR跟进了, 不过其依然没有支持grpcSimple的独特的回落到h2的功能

这可能是因为sing-box的架构与v2ray/xray的架构比较类似，都比较复杂，难以施展拳脚，而为了支持h2回落，需要一些特殊技巧。


## 开发计划

远期计划有

1. 完善并实现 vless v1协议
2. 什么时候搞一个 verysimple_c 项目，用c语言照着写一遍; 也就是说，就算本verysimple没有任何技术创新，单单架构简单也是有技术优势的，可以作为参考 实现更底层的 c语言实现。之后本以为可以加入naiveproxy，但是实际发现没那么简单.
3. verysimple_rust 项目。同上。
4. 完善 tls lazy encrypt技术
5. 链接池技术，可以重用与服务端的连接 来发起新请求
6. 握手延迟窗口技术，可用于分流一部分流量使用mux发送，达到精准降低延迟的目的；然后零星的链接依然使用单独信道。


其它开发计划请参考
https://github.com/e1732a364fed/v2ray_simple/discussions/3



## 验证方式

对于功能的golang test，请使用 `go test ./...  -count=1` 命令。如果要详细的打印出test的过程，可以添加 -v 参数

内网测试命令示例：

在 cmd/verysimple 文件夹中, 打开两个终端,
```
./verysimple -c ../../examples/quic.client.toml -ll 0
```

```
./verysimple -c ../../examples/quic.server.toml -ll 0
```

</details>


## 测速

测试环境：ubuntu虚拟机, 使用开源测试工具
https://github.com/librespeed/speedtest-go

编译后运行，会监听8989。注意要先按speedtest-go的要求，把web/asset文件夹 和一个toml配置文件 放到 可执行文件的文件夹中,我们直接在项目文件夹里编译的，所以直接移动到项目文件夹根部即可

然后内网搭建nginx 前置，加自签名证书，配置添加反代：
`proxy_pass http://127.0.0.1:8989;`
然后 speedtest-go 后置。

然后verysimple本地同时开启 客户端和 服务端，然后浏览器 firefox配置 使用 socks5代理，连到我们的verysimple客户端

注意访问测速网页时要访问https的，否则测的 splice的速度实际上还是普通的tls速度，并没有真正splice。

访问 https://自己ip/example-singleServer-full.html
注意这个自己ip不能为 127.0.0.1，因为本地回环是永远不过代理的，要配置成自己的局域网ip。

### 关于readv与测速

如果你是按上面指导内网进行测速的话，实际上readv有可能会造成减速效果，具体可参考
https://github.com/e1732a364fed/v2ray_simple/issues/14

如果发现减速，则要关闭readv

### 结果

左侧下载，右侧上传，单位Mbps。我的虚拟机性能太差，所以就算内网连接速度也很低。

不过这样正好可以测出不同代理协议之间的差距。

verysimple 版本 v1.0.3

```
//直连
156，221
163，189
165，226
162，200


//verysimple, vless v0 + tls
145，219
152，189
140，222
149，203

//verysimple, vless v0 + tls + tls lazy encrypt (splice):

161，191，
176，177
178，258
159，157
```

详细测速还可以参考另外几个文件，docs/speed_macos.md 和 docs/speed_ubuntu.md。

总之目前可以看到，verysimple是绝对的王者。虽然有时lazy还不够稳定，但是我会进一步优化这个问题的。

测速时，打开的窗口尽量少，且只留浏览器的窗口在最前方。已经证明多余的窗口会影响速率。尤其是这种消耗cpu性能的情况，在核显的电脑上确实要保证cpu其它压力减到最小。

## 交流与思想

群肯定是有的。只在此山中，云深不知处。实际上每一个群都有可能是verysimple群，每一个成员都有可能是verysimple的作者。

如果你实在找不到群，你不妨自己建一个。希望每一个人都能站出来，自豪地说，“我就是原作者”，并且能够滔滔不绝地讲解自己对verysimple的架构的理解。关键不在于谁是作者，一个作者倒下，千万个作者会站起来。

如果本作作者突然停更，这里允许任何人以 verysimple 作者的名义fork并 接盘。你只要声称自己是原作者，忘记了github和自己邮箱的密码，只好重开，这不就ok了。


# 免责声明与鸣谢

## 免责

MIT协议！作者不负任何责任。本项目 适合内网测试使用，以及适合阅读代码了解原理。

你如果用于任何其它目的，我们不会帮助你。

我们只会帮助研究理论的朋友。

同时，我们对于v2ray/xray等项目也是没有任何责任的。


## 鸣谢

为了支持hysteria 的阻塞控制，从 https://github.com/HyNetwork/hysteria 的 pkg/congestion里拷贝了 brutal.go 和 pacer.go 到我们的 quic文件夹中.

grpcSimple的客户端实现部分 借鉴了 clash 的gun的代码，该文件单独属于MIT协议。(clash的gun又是借鉴 Qv2ray的gun的）

tproxy借鉴了 https://github.com/LiamHaworth/go-tproxy/ , （trojan-go也借鉴了它; 它有数个bug, 已经都在本作修复）

来自v2ray的代码有：quic的嗅探，geosite文件的解析(v2fly/domain-list-community), vmess的 ShakeSizeParser 和 openAEADHeader 等函数。

（grpc参考了v2ray但是没直接拷贝，而是自己写的。代码看起来像 主要因为 protobuf和grpc谷歌包的特点，导致只要代码是兼容的，写出来肯定是很相似的）

以上借鉴的代码都是用的MIT协议。

vmess 的客户端代码 来自 github.com/Dreamacro/clash/transport/vmess, 使用的是 GPLv3协议。该协议直接 放在 proxy/vmess/ 文件夹下了。

同时通过该vmess 客户端代码 反推出了 对应的服务端代码。

## Stargazers over time

[![Stargazers over time](https://starchart.cc/e1732a364fed/v2ray_simple.svg)](https://starchart.cc/e1732a364fed/v2ray_simple)

