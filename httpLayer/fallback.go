package httpLayer

import (
	"errors"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
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

var ErrShouldFallback = errors.New("will fallback")

func getfallbacktype_byindex(i int) byte {
	return 1 << (i + 1)
}

//判断 Fallback.SupportType 返回的 数值 是否具有特定的Fallback类型
func HasFallbackType(ftype, b byte) bool {
	return ftype&b > 0
}

//实现 Fallback. 这里的fallback只与http协议有关，所以只能按path,alpn 和 sni 进行分类
type Fallback interface {
	GetFallback(ftype byte, params ...string) *FallbackResult
	SupportType() byte //参考Fallback_开头的常量。如果支持多个，则返回它们 按位与 的结果
}

type FallbackResult struct {
	Addr netLayer.Addr
	Xver int
}

func (ef *FallbackResult) GetFallback(ftype byte, _ ...string) *FallbackResult {
	return ef
}

func (FallbackResult) SupportType() byte {
	return FallBack_default
}

//实现 Fallback,支持 path, alpn, sni 分流。
// 内部 map 我们使用通用的集合办法, 而不是多层map嵌套;
//虽然目前就三个fallback类型，但是谁知道以后会加几个？所以这样更通用.
// 目前3种fallback性能是没问题的，不过如果 fallback继续增加的话，
// 最差情况下集合的子集总数会急剧上升,导致最差情况下性能不如多重 map;不过一般没人那么脑残会给出那种配置.
type ClassicFallback struct {
	Default FallbackResult

	supportedTypeMask byte

	Map map[FallbackConditionSet]*FallbackResult
}

func NewClassicFallback() *ClassicFallback {
	return &ClassicFallback{
		Map: make(map[FallbackConditionSet]*FallbackResult),
	}
}

type FallbackConf struct {
	//可选
	FromTag string `toml:"from" json:"from"` //which inServer triggered this fallback

	Xver int `toml:"xver" json:"xver"` //if fallback, whether to use PROXY protocol, and which version

	//必填
	Dest interface{} `toml:"dest" json:"dest"` //number port，or string "ip:port"

	//几种匹配方式，可选

	Path string   `toml:"path" json:"path"`
	Sni  string   `toml:"sni" json:"sni"`
	Alpn []string `toml:"alpn" json:"alpn"`
}

func NewClassicFallbackFromConfList(fcl []*FallbackConf) *ClassicFallback {
	cfb := NewClassicFallback()
	for _, fc := range fcl {
		addr, err := netLayer.NewAddrFromAny(fc.Dest)
		if err != nil {
			if ce := utils.CanLogErr("NewClassicFallbackFromConfList failed"); ce != nil {
				ce.Write(zap.String("netLayer.NewAddrFromAny err", err.Error()))
			}

			return nil

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

		cfb.InsertFallbackConditionSet(condition, addr, fc.Xver)

	}
	return cfb
}

func (cfb *ClassicFallback) InsertFallbackConditionSet(condition FallbackConditionSet, addr netLayer.Addr, xver int) {

	theMap := cfb.Map

	ftype := condition.GetType()
	cfb.supportedTypeMask |= ftype

	theMap[condition] = &FallbackResult{Addr: addr, Xver: xver}
}

func (cfb *ClassicFallback) SupportType() byte {

	return cfb.supportedTypeMask
}

// GetFallback 使用给出的 ftype mask 和 对应参数 来试图匹配到 回落地址.
// ss 必须按 FallBack_* 类型 从小到大顺序排列
//
func (cfb *ClassicFallback) GetFallback(ftype byte, ss ...string) *FallbackResult {
	if !HasFallbackType(cfb.supportedTypeMask, ftype) {
		return nil
	}

	if ftype == FallBack_default {
		return &cfb.Default
	}

	//log.Println("GetFallback.", cfb.supportedTypeMask, ftype, ss)

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

	/*log.Println("will check ", cd, cd.GetAllSubSets())
	for x := range cfb.Map {
		log.Println("has", x)
	}
	*/

	theMap := cfb.Map
	/*

		addr := theMap[cd]
		if addr == nil {

			ass := cd.GetAllSubSets()
			for _, v := range ass {
				//log.Println("will check ", v)
				if !HasFallbackType(cfb.supportedTypeMask, v.GetType()) {
					continue
				}

				addr = theMap[v]
				if addr != nil {
					break
				}
			}
		}*/

	addr := cd.TestAllSubSets(cfb.supportedTypeMask, theMap)

	return addr

}

// FallbackErr 可以在返回错误时，同时给定一个 指定的 Fallback
type FallbackErr interface {
	Error() string
	Fallback() Fallback
}
