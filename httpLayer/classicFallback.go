package httpLayer

import (
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
)

/*实现 Fallback,支持 path, alpn, sni 分流。
 内部 map 我们使用通用的集合办法, 而不是多层map嵌套;
虽然目前就三个fallback类型，但是谁知道以后会加几个？所以这样更通用.
 目前3种fallback性能是没问题的，不过如果 fallback继续增加的话，
 最差情况下集合的子集总数会急剧上升,导致最差情况下性能不如多重 map;不过一般没人会给出那种配置.

 TODO: 预先计算所有的子集，这样就不用每次匹配都算一遍
*/
type ClassicFallback struct {
	Default *FallbackResult

	supportedTypeMask byte

	Map map[string]map[FallbackConditionSet]*FallbackResult //第一层key为 inTag，若为 "" 则表示 来自所有inServer 的 都会被匹配
}

func NewClassicFallback() *ClassicFallback {
	cf := &ClassicFallback{
		Map: make(map[string]map[FallbackConditionSet]*FallbackResult),
	}
	cf.Map[""] = make(map[FallbackConditionSet]*FallbackResult)

	return cf
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
			// 所以这里在代码里直接就禁止这种做法就ok了
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

		cfb.InsertFallbackConditionSet(condition, fc.FromTag, addr, fc.Xver)

	}
	return cfb
}

func (cfb *ClassicFallback) InsertFallbackConditionSet(condition FallbackConditionSet, forServerTags []string, addr netLayer.Addr, xver int) {

	ftype := condition.GetType()

	if ftype == FallBack_default && len(forServerTags) == 0 {
		cfb.Default = &FallbackResult{Addr: addr, Xver: xver}
		return
	}

	cfb.supportedTypeMask |= ftype

	if len(forServerTags) == 0 {
		realMap := cfb.Map[""]
		realMap[condition] = &FallbackResult{Addr: addr, Xver: xver}

	} else {
		for _, forServerTag := range forServerTags {

			realMap := cfb.Map[forServerTag]
			if realMap == nil {
				realMap = make(map[FallbackConditionSet]*FallbackResult)
				cfb.Map[forServerTag] = realMap
			}

			realMap[condition] = &FallbackResult{Addr: addr, Xver: xver}
		}
	}

}

func (cfb *ClassicFallback) SupportType() byte {

	return cfb.supportedTypeMask
}

// GetFallback 使用给出的 ftype mask 和 对应参数 来试图匹配到 回落地址.
// ss 必须按 FallBack_* 类型 从小到大顺序排列
//
func (cfb *ClassicFallback) GetFallback(fromServerTag string, ftype byte, ss ...string) *FallbackResult {

	if ftype == FallBack_default && fromServerTag == "" {
		return cfb.Default
	}

	if !HasFallbackType(cfb.supportedTypeMask, ftype) {

		//一般都是带path的，就算是 / 根路径 也是有path的，若不匹配或没有按path的回落，则应该回落到默认回落，而不是nil
		return cfb.Default
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

	var result *FallbackResult

	realMap := cfb.Map[fromServerTag]
	if realMap == nil {
		realMap = cfb.Map[""]
	}

	if len(realMap) != 0 {
		result = cd.TestAllSubSets(cfb.supportedTypeMask, realMap)

	}

	if result == nil {
		if ftype == Fallback_path {
			return cfb.Default
		}
	}

	return result

}
