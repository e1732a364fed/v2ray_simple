
# 普通安装

下面给出安装到 ubuntu/debian amd64服务器 所需要的步骤和命令, 大家总结一下即可得到一个简单的一键脚本

本指导默认不使用root账户，且不建议用一键脚本。分步骤学习、安装 更加科学。

如果你用root账户运行的话，不要在前面加 "sudo". 

下面的命令也不要整个一大段拷贝，而要分条拷贝到终端并运行。

## 第〇步，服务器准备

首先确保自己服务器相应端口都是打开状态，防火墙要处理一下。然后安装一些BBR之类的加速组件。

## 第一步，文件部分

命令解释：先移除旧版本文件夹，然后从github下载最新版发布包，然后解压到相应位置后复制出一个配置文件。

注意，本命令只会下载正式版，不会下载 pre-release 版。如果你要测试 pre-release 版，到github上找到对应的下载链接下载，而不是使用jq所读到的版本。

注意，如果你以前用过verysimple，则最好在运行前将自己配置文件先拷贝到其它地方，防止下面代码将你原来配置误删除。
或你可以先解压 新版verysimple可执行文件 到其他位置，然后再用 mv 覆盖掉老版本 的可执行文件 和 examples 目录。

### 命令

```sh
sudo rm -rf /usr/local/etc/verysimple
sudo mkdir -p /usr/local/etc/verysimple

sudo apt-get update

sudo apt-get -y install jq curl wget

tag=`curl -sL https://api.github.com/repos/e1732a364fed/v2ray_simple/releases/latest | jq -r ".tag_name"`

wget https://github.com/e1732a364fed/v2ray_simple/releases/download/$tag/verysimple_linux_amd64.tar.xz

sudo tar -xJf verysimple_linux_amd64.tar.xz -C /usr/local/etc/verysimple

rm verysimple_linux_amd64.tar.xz

cd /usr/local/etc/verysimple

sudo cp examples/vlesss.server.toml server.toml
```


然后修改 `/usr/local/etc/verysimple/server.toml` ,使配置达到你想要的效果，注意里面证书路径要改为完整路径

你也可以不复制 配置文件，运行 `verysimple -i` 进入交互模式 生成一个你想要的配置。


## 第二步，证书部分

如果你没有证书，想要先用自签名证书试试，可以运行 `verysimple -i` 进入交互模式，然后根据提示自行生成自签名ssl证书

生成的证书会为 cert.pem 和 cert.key

当然，你也可以通过README.md 里的指导自行使用openssl生成证书。

当然，最好还是用自己的域名+acme等形式 申请真证书。

## 第三步，服务部分

### 简单方式

如果你不愿意使用linux的“后台服务”，只是想手动去令它在后台运行，那么实际上，在verysimple所在位置运行如下一段命令即可。

```
nohup ./verysimple -c server.toml >/dev/null &
```

这里将它的标准输出舍弃了，因为一般来说我们会在toml配置文件中 配置好日志文件名称；如果不舍弃输出的话，就会多一个输出（到控制台），增加系统负担。

同样，视你的权限来酌情在命令前面添加 `sudo`


### 标准方式

编辑服务文件
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

# docker 安装

查看 cmd/verysimple下 的 Dockerfile 和 docker-compose, 以及 

https://github.com/e1732a364fed/v2ray_simple/pkgs/container/v2ray_simple

主要贡献者： @qzydustin , @1Xgkr6wq

## docker

    docker pull ghcr.io/e1732a364fed/v2ray_simple:latest

    docker run -d \
    --name verysimple \
    -e TZ="Asia/Shanghai" \
    -v /dev/shm:/dev/shm \
    -v /etc/verysimple/server.toml:/etc/verysimple/server.toml \
    -v /etc/verysimple/examples:/etc/verysimple/examples \
    -v /etc/verysimple/cert.pem:/etc/verysimple/cert.pem \
    -v /etc/verysimple/cert.key:/etc/verysimple/cert.key \
    --network host \
    --restart always \
    ghcr.io/e1732a364fed/v2ray_simple:latest

这个 -v参数的话，冒号前为宿主机路径，冒号后为容器路径

这里就是映射一些 需要的文件 和 文件夹，自己修改对应的 冒号左侧 的 自己文件夹的位置。

（这个命令我没试过，如果有错误请指正）

## docker-compose

在含有 docker-compose.yaml 的目录下，运行 `docker-compose up -d` 来启动；运行 `docker-compose down` 来关闭。

这个docker-compose 设计时，要求你 宿主机有一个 `/etc/verysimple` 文件夹，里面放 一个 `server.toml` 配置文件。 其他mmdb或者 geosite文件夹 如果有需要，也可以放入 /etc/verysimple 中

你可以自行修改 该 `docker-compose.yaml` 文件

（我没试过，如果有错误请指正）
