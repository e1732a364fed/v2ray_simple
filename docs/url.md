
# vs的通用url格式 ( verysimple Standard URL Format )

vs的通用url格式，并不遵循现存市面上的其他格式，而是针对vs的架构逻辑所设计的新格式。如果你是一个vs老手，则使用vs的通用url格式你会感觉得心应手。

本通用格式既可以用于listen，也可以用于dial

注意，要想读懂本文档，需要了解url的基础知识。

https://datatracker.ietf.org/doc/html/rfc1738

https://en.wikipedia.org/wiki/URL

https://pkg.go.dev/net/url#URL

```
URI = scheme ":" ["//" authority] path ["?" query] ["#" fragment]

authority = [userinfo "@"] host [":" port]

```

## 举例

```
vlesss://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4433?insecure=true&v=0&utls=true#my_vless1

vlesss://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4433?adv=quic&v=0&extra.maxStreamsInOneConn=6&extra.congestion_control=hy&extra.mbps=1024#my_vless_quic

vmess://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4433/mypath?http=true&header.host=myhost.com

dokodemo://?target.ip=1.1.1.1&target.port=80#my_doko

shadowsocks://aes-128-gcm:mypasswordxxxxx@127.0.0.1:8000#my_ss

socks5://myusername:mypassword@127.0.0.1:1080#my_socks5_safe

socks5://127.0.0.1:1080#my_socks5 

```

## 基础部分

### scheme

即冒号前的部分，表示proxy所使用的具体协议，如 vmess, vless, shadowsocks, http, socks5, trojan 等

如果后加上了s，就表示使用tls层，比如https，或者 vlesss。注意，vless和trojan都必须要加s，才能在公网中不被发现，否则就是裸奔。任何proxy都可以加s。

### userinfo

即用户信息，存储 该proxy中能 authenticate （鉴权）某个特定用户的 信息。

熟悉ssh的同学会看出差别，ssh中，userinfo 是不包含密码的，也就是说ssh 需要 额外参数 或 步骤 来输入密码 或 等价信息 才能进行鉴权，而我们这里直接将鉴权的信息也包含在userinfo中来。

在vless/vmess中 就是 uuid, 在socks5/http中， 使用 “用户名:密码”的形式， 在shadowsocks中, 使用 method:password 的形式, 在trojan 中就是 密码。

### host:port 

就是 主机ip和端口。主机ip也可以用 域名代替。

### path

设置 http头存在时，或者用 ws的 路径 或 grpc的 service name。

举例，vmess+ws：

```
vmess://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4434/mypath?adv=ws#vmess_ws
```

那么path就是 /mypath
### fragment 

就是一个注释，标注 这个url 对于你来说的 特别含义。这个注释同样会被vs读取为tag，用于分流。

最后就是query部分，也是信息比较丰富的地方。

## query

v, 控制该proxy协议的 版本。

fallback=:80 设置回落的地址。

network 设置 使用的传输层，如 network="udp", 如不给出，则默认network为 tcp。还可以为 unix

fullcone=true设置 是否需要udp的fullcone功能。

security=aes-128-gcm  设置 vmess/ss等存在多种加密方式等proxy的 具体加密方式

adv=ws  设置使用的高级层，如不给出则没有高级层，如给出，可选 ws, grpc, quic

sendThrough=0.0.0.0:0   dial（一般为direct）设置发送时所使用的端口

### http 头相关

http=true 设置是否使用http头

http.method=GET 设置 http的method

http.version=1.1 设置http的版本

以 "header." 开头的字段：开头字母大写后，放到Header里。

比如，如果 vmess://uuid?http=true&header.host=myhost.com&header.Accept-Encoding=gzip,deflate

则会设置http头的header 的 Host 字段为 myhost.com，设置 Accept-Encoding 为 gzip,deflate, 如果有空格等情况的话，需要进行 百分号转译。

转译可以使用这个工具测试： https://www.url-encode-decode.com/

### extra 相关

在标准toml配置中，有时会填充extra项来配置额外参数。在本标准url格式中，该类配置也是相当简单的。

就是用 extra.yyy=zzz 的格式

举几个例子，比如 reject 类型的proxy 需要一个 type 的extra参数 来指示 使用什么类型的拒绝响应；
此时在url中，这么写： extra.type=nginx

再举几个例子

```
#quic
extra.maxStreamsInOneConn=6&extra.congestion_control=hy&extra.mbps=1024

#grpc (非grpcSimple的情况)
extra.grpc_multi=true
```

### tls相关 （proxy/tlsConfig.go)

insecure=true, 控制tls层是否需要严格要求 对方的真实性。

utls=true，控制是否使用utls

cert=cert.pem&key=cert.key

用于设置证书名称。

### 其他

dokodemo的目标这么写： `target.ip=1.1.1.1&target.port=80`
也可以指定network `target.network=tcp`

