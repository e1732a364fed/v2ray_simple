
# vs的通用url格式 ( verysimple Standard URL Format )

虽然verysimple通用url格式从一开始就存在，但是却没有文档详细描述（阅读proxy/creator_url.go 可以了解，毕竟golang的理念是代码就是文档。）。那么在这里就作为文档进行介绍。

vs的通用url格式，并不遵循现存市面上的其他格式，而是针对vs的架构逻辑所设计的新格式。如果你是一个vs老手，则使用vs的通用url格式你会感觉得心应手。

注意，要想读懂本文档，需要了解url的基础知识。

https://datatracker.ietf.org/doc/html/rfc1738

https://en.wikipedia.org/wiki/URL

https://pkg.go.dev/net/url#URL

```
URI = scheme ":" ["//" authority] path ["?" query] ["#" fragment]

authority = [userinfo "@"] host [":" port]

```

举例：
`vlesss://a684455c-b14f-11ea-bf0d-42010aaa0003@127.0.0.1:4433?insecure=true&v=0&utls=true#myvless1`

## 基础部分

### scheme

即冒号前的部分，表示proxy所使用的具体协议，如 vmess, vless, ss, http, socks5, trojan

如果后加上了s，就表示使用tls层，比如https，或者 vlesss。注意，vless和trojan都必须要加s，才能在公网中不被发现，否则就是裸奔。任何proxy都可以加s。

### userinfo

即用户信息，存储 该proxy中能 authenticate （鉴权）某个特定用户的 信息。

熟悉ssh的同学会看出差别，ssh中，userinfo 是不包含密码的，也就是说ssh 需要 额外参数 或 步骤 来输入密码 或 等价信息 才能进行鉴权，而我们这里直接将鉴权的信息也包含在userinfo中来。

在vless/vmess中 就是 uuid，在socks5/http中 就是 “用户名+密码”，在ss/trojan中就是 密码。

### host:port 

就是 主机ip和端口。主机ip也可以用 域名代替。

### fragment 

就是一个注释，标注 这个url 对于你来说的 特别含义。这个注释同样会被vs读取为tag，用于分流。

最后就是query部分，也是信息比较丰富的地方。

## query

v=0, 控制该proxy协议的 版本。

fallback=:80 设置回落的地址。

network 设置 使用的传输层，如 network="udp", 如不给出，则默认network为 tcp。还可以为 unix

fullcone=true设置 是否需要udp的fullcone功能。

security=aes-128-gcm 设置 vmess/ss等存在多种加密方式等proxy的 具体加密方式

adv=ws  设置使用的高级层，如不给出则没有高级层，如给出，可选 ws, grpc, quic

mux=true 是否使用内层smux

### tls相关 （proxy/tlsConfig.go)

insecure=true, 控制tls层是否需要严格要求 对方的真实性。

utls=true，控制是否使用utls

cert=cert.pem&key=cert.key

用于设置证书名称。






