macos Macbook air m1

nginx + https ，firefox,

nginx 最大上传缓存：512MB

verysimple 版本 v1.0.3

## 直连 127.0.0.1本地回环

```
8895，1329
8888，1212
```

## 直连 192.168本机局域网地址

```
8649，1084
8731，1179
```



## 客户端服务端全是 verysimple vless v0 tls

```
2960, 2280
2992, 2315
2964, 2290
```

## 客户端服务端全是 verysimple vless v0 tls,  splice (由于macos没有splice函数，所以没啥大优势，和xtls的direct类似）

```
3002, 2345
3001, 2364
2988, 2328
```


## 客户端服务端全是v2ray vless v0 tls

```
2346, 1870
2454, 1890
2413, 1840
```

## v2ray客户端 + verysimple服务端 ( vless v0 tls )

```
3031，1928
3052，1920
2846，1763
2997，1820
```

## verysimple客户端+ v2ray服务端 ( vless v0 tls )

```
2303，2095
2186，2040
2313，2107
```

## verysimple vless v1 tls

```
2914, 2251
2988, 2282
2991, 2287
```


## 客户端服务端全是xray vless tls

```
3115，2142
3109，2120
3131，2108
```

## 客户端服务端全是xray vless xtls direct

```
3061, 2226
3079, 2226
```

# 总结

在macos上，verysimple的上传速度是很强大的。不过在下载速度上略输 xray。

不过我还没有实现readv，等我加了readv后，有可能实现反超。

总之，我的双向splice功能在mac上发挥不出作用，因为mac不是linux，没splice函数。。。


# verysimple 后续测试
## v1.0.5测试

macos vless v0 +tls
1535, 1385
1536, 1398

macos, vless v0 +tls + lazy

1625, 1457
1571, 1451
1562, 1432
