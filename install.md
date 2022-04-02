
下面给出安装到 ubuntu amd64服务器 所需要的命令, 大家总结一下即可得到一个简单的一键脚本

```sh
sudo rm -rf /usr/local/etc/verysimple
sudo mkdir -p /usr/local/etc/verysimple

sudo apt-get -y install jq

tag=`curl -sL https://api.github.com/repos/hahahrfool/v2ray_simple/releases/latest | jq -r ".tag_name"`

wget https://github.com/hahahrfool/v2ray_simple/releases/download/$tag/verysimple-linux-64.tar.xz

sudo tar -xJf verysimple-linux-64.tar.xz -C /usr/local/etc/verysimple

rm verysimple-linux-64.tar.xz

cd /usr/local/etc/verysimple

sudo cp examples/vlesss.server.toml server.toml
```


然后修改 `/usr/local/etc/verysimple/server.toml` ,使配置达到你想要的效果，注意里面证书路径要改为完整路径


然后编辑服务文件
`sudo vi /etc/systemd/system/verysimple.service`

```sh

[Unit]
After=network.service

[Service]
ExecStart=/usr/local/etc/verysimple/verysimple -c /usr/local/etc/verysimple/server.toml

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

