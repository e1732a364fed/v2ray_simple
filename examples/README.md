examples 文件夹中 的 示例文件有几个必看的文件，下面按 必读顺序列出来：

    vlesss.client.toml
    vlesss.server.toml
    multi.client.toml
    multi.server.toml
    socks5.toml
    httpheader.client.toml
    httpheader.server.toml
    multi_sameport.server.toml
    vless_tproxy.client.toml

如果你使用高级层，如 ws/grpc/quic等，那你就 在阅读并掌握 上面列出 的必读示例后， 阅读 对应高级层 的示例文件。

本作的示例文件 大多数都是成对 给出的，这是便于你测试。 是的，本作提供的这些示例文件 只要是成对出现的，都是 上手就可以在内网测试的，都是监听的 127.0.0.1。

你开启两个终端，一个运行 vs服务端，一个运行 vs客户端，就能测试。