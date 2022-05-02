package httpLayer

import (
	"github.com/e1732a364fed/v2ray_simple/utils"
	"gonum.org/v1/gonum/stat/combin"
)

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

func (fcs *FallbackConditionSet) getSingleByInt(t int) (s string, b byte) {
	switch t {
	case 0:
		s = fcs.Path
	case 1:
		b = fcs.AlpnMask
	case 2:
		s = fcs.Sni

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

func (fcs *FallbackConditionSet) setSingleByInt(t int, s string, b byte) {
	switch t {
	case 0:
		fcs.Path = s
	case 1:
		fcs.AlpnMask = b

	case 2:
		fcs.Sni = s

	}
	return
}

func (fcs *FallbackConditionSet) extractSingle(t byte) (r FallbackConditionSet) {
	s, b := fcs.getSingle(t)
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
		allss := utils.AllSubSets_improve1(alltypes)

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

// TestAllSubSets 传入一个map, 对fcs自己以及其所有子集依次测试, 看是否有匹配的。
// 对比 GetAllSubSets 内存占用较大, 本方法开销则小很多, 因为1是复用内存, 2是匹配到就会返回，一般不会到遍历全部子集.
func (fcs *FallbackConditionSet) TestAllSubSets(allsupportedTypeMask byte, theMap map[FallbackConditionSet]*FallbackResult) *FallbackResult {

	if addr := theMap[*fcs]; addr != nil {
		return addr
	}

	ftype := fcs.GetType()

	// 该 FallbackConditionSet 所支持的所有类型
	alltypes := make([]byte, 0, all_non_default_fallbacktype_count)
	for thisType := byte(Fallback_path); thisType < fallback_end; thisType <<= 1 {
		if !HasFallbackType(ftype, thisType) {
			continue
		}
		alltypes = append(alltypes, thisType)
	}

	switch N := len(alltypes); N {
	case 0, 1:
		return nil
	case 2:
		if addr := theMap[fcs.extractSingle(alltypes[0])]; addr != nil {
			return addr
		}
		if addr := theMap[fcs.extractSingle(alltypes[1])]; addr != nil {
			return addr
		}

	default:

		fullbuf := make([]int, N-1)

		for K := N - 1; K > 0; K-- { //对每一种数量的组合方式进行遍历
			indexList := fullbuf[:K]

			cg := combin.NewCombinationGenerator(N, K)

		nextCombination:
			for cg.Next() { //每一个组合实例情况都判断一遍

				cg.Combination(indexList)
				curSet := FallbackConditionSet{}

				for _, typeIndex := range indexList { //按得到的type索引生成本curSet

					//有的单个元素种类不可能在map中出现过
					if !HasFallbackType(allsupportedTypeMask, getfallbacktype_byindex(typeIndex)) {
						continue nextCombination
					}

					s, b := fcs.getSingleByInt(typeIndex)
					curSet.setSingleByInt(typeIndex, s, b)
				}

				if addr := theMap[curSet]; addr != nil {
					return addr
				}
			}
		}

	}

	return nil
}
