package proxy

import (
	"strconv"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

// 用于 tproxy 或 tun/tap 这种 只有 网络层 和传输层的情况
// type LesserConf struct {
// 	Addr        string
// 	Tag         string
// 	UseSniffing bool
// 	Fullcone    bool
// }

// CommonConf is the common part of ListenConf and DialConf.
type CommonConf struct {
	Tag string `toml:"tag"` //可选

	Extra map[string]any `toml:"extra"` //用于包含任意其它数据.虽然本包自己定义的协议肯定都是已知的，但是如果其他人使用了本包的话，那就有可能添加一些 新协议 特定的数据. 而且这也便于扁平化，避免出现大量各种子块。任何子块内容都放在extra中，比如 quic的就是 extra.quic_xxx

	//tls 的最低版本号配置填在这里：
	//extra = { tls_minVersion = "1.2" }, 或 extra.tls_minVersion = "1.2"

	/////////////////// 网络层 ///////////////////

	Host string `toml:"host"` //ip 或域名. 若unix domain socket 则为文件路径
	IP   string `toml:"ip"`   //给出Host后，该项可以省略; 既有Host又有ip的情况比较适合cdn

	/////////////////// 传输层 ///////////////////

	Network string `toml:"network"` //传输层协议; 默认使用tcp, network可选值为 tcp, udp, unix;
	// 理论上来说应该用 transportLayer 作为名称，但是怕小白不懂，所以使用 network作为名称。
	// 而且也不算错，因为go的net包 也是用 network来指示 传输层/网络层协议的. 比如 net.Listen()第一个参数可以用 ip, tcp, udp 等。

	Sockopt *netLayer.Sockopt `toml:"sockopt"` //可选

	Port int `toml:"port"` //若Network不为 unix , 则port项必填

	Xver int `toml:"xver"` //可选，只能为0/1/2. 若不为0, 则使用 PROXY protocol 协议头.

	Fullcone bool `toml:"fullcone"` //在udp会用到, fullcone的话因为不能关闭udp连接, 所以 时间长后, 可能会导致too many open files. fullcone 的话一般人是用不到的, 所以 有需要的人自行手动打开 即可

	/////////////////// tls层 ///////////////////

	TLS      bool     `toml:"tls"`      //tls层; 可选. 如果不使用 's' 后缀法，则还可以配置这一项来更清晰地标明使用tls
	TlsType  string   `toml:"tls_type"` //可选，可以为 utls或者shadowTls, 若不给出或为空, 则为golang的标准tls. utls 只在客户端有效。
	Insecure bool     `toml:"insecure"` //tls 是否安全
	Alpn     []string `toml:"alpn"`

	TLSCert string `toml:"cert"` //可选
	TLSKey  string `toml:"key"`  //可选

	Lazy bool `toml:"lazy"` //可选, 是否开启 tls_lazy_encrypt 功能

	/////////////////// http层 ///////////////////

	HttpHeader *httpLayer.HeaderPreset `toml:"header"` //http伪装头; 可选

	Path string `toml:"path"` //ws 的path 或 grpc的 serviceName。为了简便我们在同一位置给出.

	/////////////////// 高级层 ///////////////////

	AdvancedLayer string `toml:"adv"`   //高级层; 可选
	IsEarly       bool   `toml:"early"` //是否启用 0-rtt

	/////////////////// 代理层 ///////////////////

	Protocol    string `toml:"protocol"`     //代理层; 约定，如果一个Protocol尾缀去掉了一个's'后仍然是一个有效协议，则该协议使用了 tls。这种方法继承自 v2simple，适合极简模式
	Uuid        string `toml:"uuid"`         //代理层用户的唯一标识，视代理层协议而定，一般使用uuid，但trojan协议是随便的password, 而socks5 和 http 则使用 user+pass 的形式。 我们为了简洁、一致，就统一放到了 这个字段里。
	Version     int    `toml:"version"`      //可选，代理层协议版本号，vless v1 要用到。
	EncryptAlgo string `toml:"encrypt_algo"` //内部加密算法，vmess/ss 等协议可指定

}

// 和 GetAddrStrForListenOrDial 的区别是，它优先使用host，其次再使用ip
func (cc *CommonConf) GetAddrStr() string {
	switch cc.Network {
	case "unix":
		return cc.Host

	default:
		if cc.Host != "" {

			return cc.Host + ":" + strconv.Itoa(cc.Port)
		} else {
			return cc.IP + ":" + strconv.Itoa(cc.Port)

		}

	}

}

// if network is unix domain socket, return Host，or return ip:port / host:port; 和 GetAddr的区别是，它优先使用ip，其次再使用host
func (cc *CommonConf) GetAddrStrForListenOrDial() string {
	switch cc.Network {
	case "unix":
		return cc.Host

	default:
		if cc.IP != "" {
			return cc.IP + ":" + strconv.Itoa(cc.Port)

		} else {
			return cc.Host + ":" + strconv.Itoa(cc.Port)

		}

	}

}

// config for listening, the user can be called as listener or inServer.
//
//	CommonConf.Host , CommonConf.IP, CommonConf.Port is the addr and port for listening
type ListenConf struct {
	CommonConf

	Users []utils.UserConf `toml:"users"` //可选, 用于储存多个用户/密码 信息。

	CA string `toml:"ca"` //可选,用于 验证"客户端证书"

	SniffConf *SniffConf `toml:"sniffing"` //用于嗅探出 host 来帮助 分流。

	Fallback any `toml:"fallback"` //可选，默认回落的地址，一般可为 ip:port,数字port or unix socket的文件名

	//noroute 意味着 传入的数据 不会被分流，一定会被转发到默认的 dial
	// 这一项是针对 分流功能的. 如果不设noroute, 则所有listen 得到的流量都会被 试图 进行分流
	NoRoute bool `toml:"noroute"`

	TargetAddr string `toml:"target"` //若使用dokodemo协议，则这一项会给出. 格式为url, 如 tcp://127.0.0.1:443 , 必须带scheme，以及端口。只能为tcp或udp

}

// config for dialing, user can be called dialer or outClient.
//
//	CommonConf.Host , CommonConf.IP, CommonConf.Port  are the addr and port for dialing.
type DialConf struct {
	CommonConf

	SendThrough string `toml:"sendThrough"` //可选，用于发送数据的 IP 地址, 可以是ip:port, 或者 tcp:ip:port\nudp:ip:port
	Mux         bool   `toml:"use_mux"`     //是否使用内层mux。在某些支持mux命令的协议中（vless v1/trojan）, 开启此开关会让 dial 使用 内层mux。
}

type SniffConf struct {
	Enable bool `toml:"enabled"`
}
