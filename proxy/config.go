package proxy

import (
	"strconv"
)

// CommonConf 是标准配置中 Listen和Dial 都有的部分
//如果新协议有其他新项，可以放入 Extra.
type CommonConf struct {
	Tag      string   `toml:"tag"`      //可选
	Protocol string   `toml:"protocol"` //约定，如果一个Protocol尾缀去掉了's'后仍然是一个有效协议，则该协议使用了 tls。这种方法继承自 v2simple，适合极简模式
	Uuid     string   `toml:"uuid"`     //一个用户的唯一标识，建议使用uuid，但也不一定
	Host     string   `toml:"host"`     //ip 或域名. 若unix domain socket 则为文件路径
	IP       string   `toml:"ip"`       //给出Host后，该项可以省略; 既有Host又有ip的情况比较适合cdn
	Port     int      `toml:"port"`     //若Network不为 unix , 则port项必填
	Version  int      `toml:"version"`  //可选
	TLS      bool     `toml:"tls"`      //可选. 如果不使用 's' 后缀法，则还可以配置这一项来更清晰第标明使用tls
	Insecure bool     `toml:"insecure"` //tls 是否安全
	Alpn     []string `toml:"alpn"`

	Network string `toml:"network"` //默认使用tcp, network可选值为 tcp, udp, unix;

	AdvancedLayer string `toml:"advancedLayer"` //可不填，或者为ws，或者为grpc

	Path string `toml:"path"` //ws 的path 或 grpc的 serviceName。为了简便我们在同一位置给出.

	Extra map[string]interface{} `toml:"extra"` //用于包含任意其它数据.虽然本包自己定义的协议肯定都是已知的，但是如果其他人使用了本包的话，那就有可能添加一些 新协议 特定的数据.
}

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

//若为unix, 返回Host，否则返回 ip:port / host:port; 和 GetAddr的区别是，它优先使用ip，其次再使用host
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

// 监听所使用的设置, 使用者可被称为 listener 或者 inServer
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为监听地址与端口
type ListenConf struct {
	CommonConf
	Fallback any    `toml:"fallback"` //可选，默认回落的地址，一般可以是ip:port,数字port 或者 unix socket的文件名
	TLSCert  string `toml:"cert"`
	TLSKey   string `toml:"key"`

	//noroute 意味着 传入的数据 不会被分流，一定会被转发到默认的 dial
	// 这一项是针对 分流功能的. 如果不设noroute, 则所有listen 得到的流量都会被 试图 进行分流
	NoRoute bool `toml:"noroute"`

	TargetAddr string `toml:"target"` //若使用dokodemo协议，则这一项会给出. 格式为url, 如 tcp://127.0.0.1:443 , 必须带scheme，以及端口。只能为tcp或udp

}

// 拨号所使用的设置, 使用者可被称为 dialer 或者 outClient
//  CommonConf.Host , CommonConf.IP, CommonConf.Port  为拨号地址与端口
type DialConf struct {
	CommonConf
	Utls     bool `toml:"utls"`
	Fullcone bool `toml:"fullcone"` //在direct会用到, fullcone的话因为不能关闭udp连接, 所以可能会导致too many open files. fullcone 的话一般人是用不到的, 所以 有需要的人自行手动打开 即可
}
