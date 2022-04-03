
下面给出安装到 ubuntu amd64服务器 所需要的命令, 大家总结一下即可得到一个简单的一键脚本

## 第一步，文件部分
```sh
sudo rm -rf /usr/local/etc/verysimple
sudo mkdir -p /usr/local/etc/verysimple

sudo apt-get -y install jq

tag=`curl -sL https://api.github.com/repos/hahahrfool/v2ray_simple/releases/latest | jq -r ".tag_name"`

wget https://github.com/hahahrfool/v2ray_simple/releases/download/$tag/verysimple_linux_amd64.tar.xz

sudo tar -xJf verysimple_linux_amd64.tar.xz -C /usr/local/etc/verysimple

rm verysimple_linux_amd64.tar.xz

cd /usr/local/etc/verysimple

sudo cp examples/vlesss.server.toml server.toml
```


然后修改 `/usr/local/etc/verysimple/server.toml` ,使配置达到你想要的效果，注意里面证书路径要改为完整路径


## 第二部，证书部分

如果你没有证书，想要先用自签名证书试试，可以运行 `verysimple -i` 进入交互模式，然后根据提示自行生成tls证书

生成的证书会为 yourcert.pem 和 yourcert.key

你可以重命名为 cert.pem 和 cert.key 来匹配示例文件，或者反过来，修改示例文件里的证书名称以匹配 yourcert这个名称.


## 第三步，服务部分
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

然后运行下面命令来启动verysimple服务

```sh
sudo chmod 664 /etc/systemd/system/verysimple.service

sudo systemctl daemon-reload
sudo systemctl enable verysimple
sudo systemctl start verysimple
```

