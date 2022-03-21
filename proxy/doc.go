/*Package proxy 定义了代理转发所需的必备组件.

Layer Definition

目前认为，一个传输过程由四个部分组成，基础连接（udp/tcp），TLS（可选），中间层（ws、grpc、http等，可选），具体协议（socks5，vless，trojan等）.

其中，ws和grpc被认为是 高级应用层，http（伪装）属于低级应用层.

TLS - Transport Layer Security 顾名思义TLS作用于传输层，第四层，但是我们tcp也是第四层，所以在本项目中，认为不需要“会话层”，单独加一个专用于tls的层比较稳妥.

正常OSI是7层，我们在这里规定一个 第八层和第九层，第八层就是 vless协议所在位置，第九层就是我们实际传输的承载数据.


New Model - VSI 新的VSI 模型

那么我们提出一个 verysimple Interconnection Model， 简称vsi模型。1到4层与OSI相同（物理、链路、网络、传输).

把第五层替换成“加密层”，把TLS放进去；把第六层改为低级应用层，http属于这一层.

第七层 改为高级应用层，ws/grpc 属于这一层, 简称高级层；第八层定为 代理层，vless/trojan 在这层.

第九层为 承载数据层，承载的为 另一大串 第四层的数据.

那么我们verysimple实际上就是 基于 “层” 的架构，或称 可分层结构.


	9 ｜ tcp data
	--------------------
	8 ｜ vless/trojan/socks5
	--------------------
	7 ｜ ws/grpc
	--------------------
	6 ｜ http
	--------------------
	5 ｜ tls
	--------------------
	4 ｜ tcp
	--------------------


基本上5-8层都是可控的.

对应的理想配置文件应该如下.

	{
		"layer5_settings": {	//或者叫 tls_settings，
			"tls":{"insecure": true},
			"utls":{}
		},
		"layer6_settings": {	//或者叫 http_settings
			"headers":{}
		},
		"layer7_settings": {	//或者叫 advancedLayer_settings
			"ws":{}
		},
		"layer8_settings": {	//或者叫 proxy_settings
			"vless":{}
			"trojan":{}
		},
	}

我们项目的文件夹，netLayer 第3，4层，tlsLayer文件夹代表第5层; httpLayer第六层，
ws和grpc文件夹（第七层）proxy文件夹代表第8层.


同级的ws和grpc是独占的，可以都放到一个layer里，然后比如第八层配置了一个vless一个trojan，那么排列组合就是4种，vless+ws, vless+ grpc, trojan+ws, trojan+grpc.


这就大大减轻了各种”一键脚本“的 使用需求，咱们只要选择自己喜欢的各个层，程序自动就为我们生成所有配置.

运行时，如果所有配置都要有，那么就需要多种端口；共用端口的话可以用nginx.

也可以程序指定一种 特定的情况，比如开始运行程序时，冒出交互界面，自己按项选择好后，就自动运行，然后自动生成客户端分享url.

可以在脑海里想象 “穿鞋带” 的画面，有很多洞可以经过，都穿好了，鞋带就系好了。或者手机手势解锁的情况.

这种好处是，每次运行都可以采用不同的配置，不同的uuid，手机一扫码就能连上.

然而，这种“高级模式”是不容易实现，也不好理解的，目前初始阶段先不考虑。

目前的标准模式的配置文件中，整个一个节点的配置完全是扁平化的，所有的层的配置都会在同一级别中。比如tls的配置完全和节点本身的配置放在一起。总之我的思路就是，要不就完全扁平化，要不就完全分层。

本作认为，所有的代理是都可以有tls层，http层和ws/grpc层的，所以就统一嵌入所有的代理协议的配置当中,直接扁平化了.

Contents of proxy package - proxy包内容

接口 ProxyCommon 和 结构 ProxyCommonStruct 给 这个架构定义了标准.

而 Client 和 Server 接口 是 具体利用该架构的 客户端 和 服务端，都位于VSI中的第八层.

使用 RegisterClient 和 RegisterServer 来注册新的实现.

还定义了关于udp 转发到机制，该部分直接参考 各个UDP开头的 部分即可.

Server and Client

我们服务端和 客户端的程序，都是有至少一个入口和一个出口的。入口我们叫做 inServer ，出口我们叫做 outClient.

这两个词的含义和 v2ray的 inbound 和 outbound 是等价的.

在 inServer 中，我们负责监听未知连接；在 outClient 中，我们负责拨号特定目标服务器.

可以用 两种Creator来创建Server和Client.

CreateClientFromMap 和 CreateClientFromURL 实际上是等价的, FromMap 专门支持从json, toml等配置文件中加载.

而且一般来说URL只适用于简单配置，复杂的配置还是要使用到map.

*/
package proxy
