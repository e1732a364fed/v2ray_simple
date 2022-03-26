
下面给出安装到 ubuntu amd64服务器 所需要的命令, 大家总结一下即可得到一个简单的一键脚本


```sh
wget https://github.com/hahahrfool/v2ray_simple/releases/download/v1.0.9/verysimple_linux_amd64_v1.0.9.tgz

tar -xzf verysimple_linux_amd64_v1.0.9.tgz
sudo mkdir -p /usr/local/etc/verysimple
sudo mv verysimple /usr/local/etc/verysimple/
sudo mv vlesss.server.toml /usr/local/etc/verysimple/server.toml

```


然后修改 `/usr/local/etc/verysimple/server.toml` ,使配置达到你想要的效果


然后编辑服务文件
`sudo vi /etc/systemd/system/verysimple.service`

```sh

[Unit]
After=network.service

[Service]
ExecStart=/usr/local/etc/verysimple/verysimple -c server.toml

[Install]
WantedBy=default.target
```

然后运行下面命令
```sh
sudo chmod 664 /etc/systemd/system/verysimple.service

sudo systemctl daemon-reload
sudo systemctl enable verysimple
sudo systemctl start verysimple
```

