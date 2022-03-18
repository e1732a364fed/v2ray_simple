package httpLayer

import (
	"bytes"
	"log"
	"reflect"
	"strconv"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

const (
	Fallback_none = 0
)
const (
	FallBack_default byte = 1 << iota
	Fallback_alpn
	Fallback_path
	Fallback_sni
)

//判断 Fallback.SupportType 返回的 数值 是否具有特定的Fallback类型
func HasFallbackType(ftype, b byte) bool {
	return ftype&b > 0
}

//实现 Fallback. 这里的fallback只与http协议有关，所以只能按path,alpn 和 sni 进行分类
type Fallback interface {
	GetFallback(ftype byte, param string) *netLayer.Addr
	SupportType() byte          //参考Fallback_开头的常量。如果支持多个，则返回它们 按位与 的结果
	FirstBuffer() *bytes.Buffer //因为能确认fallback一定是读取过数据的，所以需要给出之前所读的数据，fallback时要用到，要重新传输给目标服务器
}

type SingleFallback struct {
	Addr  *netLayer.Addr
	First *bytes.Buffer
}

func (ef *SingleFallback) GetFallback(ftype byte, param string) *netLayer.Addr {
	return ef.Addr
}

func (ef *SingleFallback) SupportType() byte {
	return FallBack_default
}

func (ef *SingleFallback) FirstBuffer() *bytes.Buffer {
	return ef.First
}

//实现 Fallback,支持 path,alpn, sni 分流
type ClassicFallback struct {
	First     *bytes.Buffer
	Default   *netLayer.Addr
	MapByPath map[string]*netLayer.Addr //因为只一次性设置，之后仅用于读，所以不会有多线程问题
	MapByAlpn map[string]*netLayer.Addr
	MapBySni  map[string]*netLayer.Addr
}

func NewClassicFallback() *ClassicFallback {
	return &ClassicFallback{
		MapByPath: make(map[string]*netLayer.Addr),
		MapByAlpn: make(map[string]*netLayer.Addr),
		MapBySni:  make(map[string]*netLayer.Addr),
	}
}

type FallbackConf struct {
	Path string      `json:"path"`
	Dest interface{} `json:"dest"` //可为数字端口号，或者 字符串的ip:port
}

func NewClassicFallbackFromConfList(fcl []*FallbackConf) *ClassicFallback {
	cfb := NewClassicFallback()
	for _, v := range fcl {
		//log.Println("NewClassicFallbackFromConfList called", reflect.TypeOf(v.Dest))
		//json 默认把数字转换成float64，就算是整数也一样
		var integer int

		if thefloat, ok := v.Dest.(float64); ok {

			log.Println("got num", thefloat)
			if thefloat > 65535 || thefloat < 1 {
				log.Println("int port not valid", thefloat)

				continue
			}

			integer = int(thefloat)

		} else if theInt64, ok := v.Dest.(int64); ok { //toml 可把数字加载为int64
			integer = int(theInt64)
		} else {
			if utils.CanLogErr() {
				log.Println("Fallback port config type err", reflect.TypeOf(v.Dest))
			}
			continue
		}

		addr, e := netLayer.NewAddr("127.0.0.1:" + strconv.Itoa(integer))
		if e != nil {
			log.Fatalln("addr create failed", e, strconv.Itoa(integer))
		}
		cfb.MapByPath[v.Path] = addr
	}
	return cfb
}

func (ef *ClassicFallback) FirstBuffer() *bytes.Buffer {
	return ef.First
}
func (ef *ClassicFallback) SupportType() byte {
	var r byte = 0

	if ef.Default != nil {
		r |= FallBack_default
	}

	if len(ef.MapByAlpn) != 0 {
		r |= Fallback_alpn
	}

	if len(ef.MapByPath) != 0 {
		r |= Fallback_path
	}

	if len(ef.MapBySni) != 0 {
		r |= Fallback_sni
	}

	return FallBack_default
}

func (ef *ClassicFallback) GetFallback(ftype byte, s string) *netLayer.Addr {
	switch ftype {
	default:
		return ef.Default
	case Fallback_path:
		return ef.MapByPath[s]
	case Fallback_alpn:
		return ef.MapByAlpn[s]
	case Fallback_sni:
		return ef.MapBySni[s]
	}

}

type FallbackErr interface {
	Error() string
	Fallback() Fallback
}

//实现 FallbackErr
type ErrSingleFallback struct {
	FallbackAddr *netLayer.Addr
	Err          error
	eStr         string
	First        *bytes.Buffer
}

func (ef *ErrSingleFallback) Error() string {
	if ef.eStr == "" {
		ef.eStr = ef.Err.Error() + ", default fallback is " + ef.FallbackAddr.String()
	}
	return ef.eStr
}

//返回 SingleFallback
func (ef *ErrSingleFallback) Fallback() Fallback {
	return &SingleFallback{
		Addr:  ef.FallbackAddr,
		First: ef.First,
	}
}
