package httpLayer

import (
	"bytes"

	"github.com/hahahrfool/v2ray_simple/proxy"
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
	GetFallback(ftype byte, param string) *proxy.Addr
	SupportType() byte //参考Fallback_开头的常量。如果支持多个，则返回它们 按位与 的结果
	FirstBuffer() *bytes.Buffer
}

type SingleFallback struct {
	Addr  *proxy.Addr
	First *bytes.Buffer
}

func (ef *SingleFallback) GetFallback(ftype byte, param string) *proxy.Addr {
	return ef.Addr
}

func (ef *SingleFallback) SupportType() byte {
	return FallBack_default
}

func (ef *SingleFallback) FirstBuffer() *bytes.Buffer {
	return ef.First
}

//实现 Fallback
type ClassicFallback struct {
	Default   *proxy.Addr
	MapByPath map[string]*proxy.Addr //因为只一次性设置，之后仅用于读，所以不会有多线程问题
	MapByAlpn map[string]*proxy.Addr
	MapBySni  map[string]*proxy.Addr
}

func NewClassicFallback() *ClassicFallback {
	return &ClassicFallback{
		MapByPath: make(map[string]*proxy.Addr),
		MapByAlpn: make(map[string]*proxy.Addr),
		MapBySni:  make(map[string]*proxy.Addr),
	}
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

func (ef *ClassicFallback) GetFallback(ftype byte, s string) *proxy.Addr {
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
	FallbackAddr *proxy.Addr
	Err          error
	eStr         string
	First        *bytes.Buffer
}

func (ef *ErrSingleFallback) Error() string {
	if ef.eStr == "" {
		ef.eStr = ef.Err.Error() + ", and will fallback to " + ef.FallbackAddr.String()
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
