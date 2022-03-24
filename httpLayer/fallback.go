package httpLayer

import (
	"bytes"
	"log"

	"github.com/hahahrfool/v2ray_simple/netLayer"
	"github.com/hahahrfool/v2ray_simple/utils"
)

const (
	Fallback_none = 0
)
const (
	FallBack_default byte = 1 << iota

	//这里剩余fallback按该固定顺序排列. 这是为了方便代码的书写(alpn和sni都是tls的)

	// 虽然alpn和sni都是tls的，但是回落本身是专门用于http的，所以还是放在httpLayer包中

	Fallback_path
	Fallback_alpn
	Fallback_sni

	fallback_end

	all_non_default_fallbacktype_count = iota - 2

	alpn_unspecified = 0
)

const (
	alpn_http11 = 1 << iota
	alpn_http20
)

//判断 Fallback.SupportType 返回的 数值 是否具有特定的Fallback类型
func HasFallbackType(ftype, b byte) bool {
	return ftype&b > 0
}

//实现 Fallback. 这里的fallback只与http协议有关，所以只能按path,alpn 和 sni 进行分类
type Fallback interface {
	GetFallback(ftype byte, params ...string) *netLayer.Addr
	SupportType() byte          //参考Fallback_开头的常量。如果支持多个，则返回它们 按位与 的结果
	FirstBuffer() *bytes.Buffer //因为能确认fallback一定是读取过数据的，所以需要给出之前所读的数据，fallback时要用到，要重新传输给目标服务器
}

type SingleFallback struct {
	Addr  *netLayer.Addr
	First *bytes.Buffer
}

func (ef *SingleFallback) GetFallback(ftype byte, _ ...string) *netLayer.Addr {
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
	First   *bytes.Buffer
	Default *netLayer.Addr

	supportedTypeMask byte

	Map map[byte]map[FallbackConditionSet]*netLayer.Addr
}

type FallbackConditionSet struct {
	Path, Sni string
	AlpnMask  byte
}

func (fcs *FallbackConditionSet) GetType() (r byte) {
	if fcs.Path != "" {
		r |= (Fallback_path)
	}
	if fcs.Sni != "" {
		r |= Fallback_sni
	}
	if fcs.AlpnMask > 0 {
		r |= Fallback_alpn
	}
	return r
}

func (fcs *FallbackConditionSet) GetSub(subType byte) (r FallbackConditionSet) {

	for thisType := byte(Fallback_path); thisType < fallback_end; thisType++ {
		if !HasFallbackType(subType, thisType) {
			continue
		}
		switch thisType {
		case Fallback_alpn:
			r.AlpnMask = fcs.AlpnMask

		case Fallback_path:
			r.Path = fcs.Path
		case Fallback_sni:
			r.Sni = fcs.Sni
		}
	}
	return
}

func (fcs *FallbackConditionSet) getSingle(t byte) (s string, b byte) {
	switch t {
	case Fallback_path:
		s = fcs.Path
	case Fallback_sni:
		s = fcs.Sni
	case Fallback_alpn:
		b = fcs.AlpnMask
	}
	return
}

func (fcs *FallbackConditionSet) setSingle(t byte, s string, b byte) {
	switch t {
	case Fallback_sni:
		fcs.Sni = s
	case Fallback_path:
		fcs.Path = s
	case Fallback_alpn:
		fcs.AlpnMask = b
	}
	return
}

func (fcs *FallbackConditionSet) extractSingle(t byte) (r FallbackConditionSet) {
	s, b := fcs.getSingle(t)
	r = FallbackConditionSet{}
	r.setSingle(t, s, b)
	return
}

//返回不包括自己的所有子集
func (fcs *FallbackConditionSet) GetAllSubSets() (rs []FallbackConditionSet) {

	alltypes := make([]byte, 0, all_non_default_fallbacktype_count)
	ftype := fcs.GetType()
	for thisType := byte(Fallback_path); thisType < fallback_end; thisType <<= 1 {
		if !HasFallbackType(ftype, thisType) {
			continue
		}
		alltypes = append(alltypes, thisType)
	}

	switch len(alltypes) {
	case 0, 1:

		return nil

	case 2:
		rs = make([]FallbackConditionSet, 2)
		rs[0] = fcs.extractSingle(alltypes[0])
		rs[1] = fcs.extractSingle(alltypes[1])

		return
	default:
		allss := utils.AllSubSets(alltypes)

		rs = make([]FallbackConditionSet, len(allss))
		for i, sList := range allss {
			for _, singleType := range sList {
				s, b := fcs.getSingle(singleType)
				rs[i].setSingle(singleType, s, b)
			}
		}
	}

	return
}

func NewClassicFallback() *ClassicFallback {
	return &ClassicFallback{
		Map: make(map[byte]map[FallbackConditionSet]*netLayer.Addr),
	}
}

type FallbackConf struct {
	//必填
	Dest interface{} `toml:"dest" json:"dest"` //可为数字端口号，或者 字符串的ip:port

	//几种匹配方式，可选

	Path string   `toml:"path" json:"path"`
	Sni  string   `toml:"sni" json:"sni"`
	Alpn []string `toml:"alpn" json:"alpn"`
}

func NewClassicFallbackFromConfList(fcl []*FallbackConf) *ClassicFallback {
	cfb := NewClassicFallback()
	for _, fc := range fcl {
		//log.Println("NewClassicFallbackFromConfList called", reflect.TypeOf(v.Dest))

		addr, err := netLayer.NewAddrFromAny(fc.Dest)
		if err != nil {
			log.Fatal(err)
		}
		var aMask byte
		if len(fc.Alpn) > 2 {
			//理论上alpn可以为任意值，但是由于我们要回落，搞那么多奇葩的alpn只会增加被审查的概率
			// 所以这里在代码端直接就禁止这种做法就ok了
		} else {
			for _, v := range fc.Alpn {
				if v == H2_Str {
					aMask |= alpn_http20
				} else if v == H11_Str {
					aMask |= alpn_http11
				}
			}
		}

		condition := FallbackConditionSet{
			Path:     fc.Path,
			Sni:      fc.Sni,
			AlpnMask: aMask,
		}

		cfb.InsertFallbackConditionSet(condition, addr)

	}
	return cfb
}

func (cfb *ClassicFallback) InsertFallbackConditionSet(condition FallbackConditionSet, addr *netLayer.Addr) {
	ctype := condition.GetType()

	theMap := cfb.Map[ctype]
	if theMap == nil {
		theMap = make(map[FallbackConditionSet]*netLayer.Addr)
		cfb.Map[ctype] = theMap
		cfb.supportedTypeMask |= ctype
	}
	theMap[condition] = addr
}

func (cfb *ClassicFallback) FirstBuffer() *bytes.Buffer {
	return cfb.First
}
func (cfb *ClassicFallback) SupportType() byte {

	return cfb.supportedTypeMask
}

// ss 必须按 FallBack_* 类型 从小到大顺序排列
//
func (cfb *ClassicFallback) GetFallback(ftype byte, ss ...string) *netLayer.Addr {
	if !HasFallbackType(cfb.supportedTypeMask, ftype) {
		return nil
	}

	if ftype == FallBack_default {
		return cfb.Default
	}

	theMap := cfb.Map[ftype]
	if theMap == nil || len(theMap) == 0 {
		return nil
	}
	cd := FallbackConditionSet{}

	ss_cursor := 0

	for thisType := byte(Fallback_path); thisType < fallback_end; thisType <<= 1 {
		if len(ss) <= ss_cursor {
			break
		}
		if !HasFallbackType(ftype, thisType) {
			continue
		}

		param := ss[ss_cursor]
		ss_cursor++

		if param == "" {
			continue
		}
		switch thisType {
		case Fallback_alpn:
			var aMask byte
			if param == H11_Str {
				aMask |= alpn_http11
			}
			if param == H2_Str {
				aMask |= alpn_http20
			}

			cd.AlpnMask = aMask
		case Fallback_path:
			cd.Path = param
		case Fallback_sni:
			cd.Sni = param
		}
	}
	addr := theMap[cd]
	if addr == nil {

		ass := cd.GetAllSubSets()
		for _, v := range ass {
			addr = theMap[v]
			if addr != nil {
				break
			}
		}
	}

	return addr

}

// FallbackErr 可以在返回错误时，同时给定一个 指定的 Fallback
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
