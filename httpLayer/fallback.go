package httpLayer

import (
	"bytes"

	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"go.uber.org/zap"
	"gonum.org/v1/gonum/stat/combin"
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

func getfallbacktype_byindex(i int) byte {
	return 1 << (i + 1)
}

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
// 内部 map 我们使用通用的集合办法, 而不是多层map嵌套;
//虽然目前就三个fallback类型，但是谁知道以后会加几个？所以这样更通用.
// 目前3种fallback性能是没问题的，不过如果 fallback继续增加的话，
// 最差情况下集合的子集总数会急剧上升,导致最差情况下性能不如多重 map;不过一般没人那么脑残会给出那种配置.
type ClassicFallback struct {
	First   *bytes.Buffer
	Default *netLayer.Addr

	supportedTypeMask byte

	Map map[FallbackConditionSet]*netLayer.Addr
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
func (fcs *FallbackConditionSet) TestAllSubSets(allsupportedTypeMask byte, theMap map[FallbackConditionSet]*netLayer.Addr) *netLayer.Addr {

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

func NewClassicFallback() *ClassicFallback {
	return &ClassicFallback{
		Map: make(map[FallbackConditionSet]*netLayer.Addr),
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

		cfb.InsertFallbackConditionSet(condition, addr)

	}
	return cfb
}

func (cfb *ClassicFallback) InsertFallbackConditionSet(condition FallbackConditionSet, addr netLayer.Addr) {

	theMap := cfb.Map

	ftype := condition.GetType()
	cfb.supportedTypeMask |= ftype

	theMap[condition] = &addr
}

func (cfb *ClassicFallback) FirstBuffer() *bytes.Buffer {
	return cfb.First
}
func (cfb *ClassicFallback) SupportType() byte {

	return cfb.supportedTypeMask
}

// GetFallback 使用给出的 ftype mask 和 对应参数 来试图匹配到 回落地址.
// ss 必须按 FallBack_* 类型 从小到大顺序排列
//
func (cfb *ClassicFallback) GetFallback(ftype byte, ss ...string) *netLayer.Addr {
	if !HasFallbackType(cfb.supportedTypeMask, ftype) {
		return nil
	}

	if ftype == FallBack_default {
		return cfb.Default
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
