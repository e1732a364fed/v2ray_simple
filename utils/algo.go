package utils

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

func ArrayToPtrArray[T any](a []T) (r []*T) {
	for _, v := range a {
		r = append(r, &v)
	}
	return
}

//Combinatorics ////////////////////////////////////////////////////////////////

// func AllSubSets edited from https://github.com/mxschmitt/golang-combinations with MIT License
// All returns all combinations for a given T array.
// This is essentially a powerset of the given set except that the empty set is disregarded.
func AllSubSets[T comparable](set []T) (subsets [][]T) {
	length := uint(len(set))

	// Go through all possible combinations of objects
	// from 1 (only first object in subset) to 2^length (all objects in subset)
	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []T

		for object := uint(0); object < length; object++ {
			// checks if object is contained in subset
			// by checking if bit 'object' is set in subsetBits
			if (subsetBits>>object)&1 == 1 {
				// add object to subset
				subset = append(subset, set[object])
			}
		}
		// add subset to subsets
		subsets = append(subsets, subset)
	}
	return subsets
}

// AllSubSets 测速有点慢, 我改进一下内存分配,可加速一倍多
func AllSubSets_improve1[T comparable](set []T) (subsets [][]T) {
	length := uint(len(set))
	subsets = make([][]T, 0, length*length)

	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []T = make([]T, 0, length)

		for object := uint(0); object < length; object++ {
			if (subsetBits>>object)&1 == 1 {
				subset = append(subset, set[object])
			}
		}
		subsets = append(subsets, subset)
	}
	return subsets
}

// generics ////////////////////////////////////////////////////////////////

func CloneSlice[T any](a []T) (r []T) {
	r = make([]T, len(a))
	copy(r, a)
	return

	//实际上 golang.org/x/exp/slices 的 Clone 函数也可以
}

// TrimSlice 从一个slice中移除一个元素, 会直接改动原slice数据
func TrimSlice[T any](a []T, deleteIndex int) []T {
	j := 0
	for idx, val := range a {
		if idx != deleteIndex {
			a[j] = val
			j++
		}
	}
	return a[:j]

	//实际上 golang.org/x/exp/slices 的 Delete 函数也可以
}

// 根据传入的order来对arr重新排序；order必须长度与arr一致，而且包含所有索引
// 若erri>0 则证明传入的order内容有误。1表示过长，2表示过短，3表示内容出错.
// 在 erri>0 时，本函数会试图修复order，生成一个neworder并用该 neworder 对arr排序。
func SortByOrder[T any](arr []T, order []int) (result []T, neworder []int, erri int) {
	//检查长度
	if len(order) != len(arr) {
		if len(order) > len(arr) { //这种是有问题的，不应传入;不过，我们整理出一个新的order列表

			erri = 1
			neworder = make([]int, 0, len(arr))

			//填上已有的
			for _, v := range order {
				if v >= 0 && v < len(arr) && !slices.Contains(neworder, v) {
					neworder = append(neworder, v)
				}
			}

			//补全没有的
			for i := 0; i < len(arr); i++ {
				if !slices.Contains(neworder, i) {
					neworder = append(neworder, i)
				}
			}

			order = neworder
		} else {
			erri = 2

			//补全没有的

			for i := 0; i < len(arr); i++ {
				if !slices.Contains(order, i) {
					order = append(order, i)
				}
			}
			neworder = order
		}
	}
	//检查重复或索引不正确
	for i, v := range order {
		order[i] = -1
		if slices.Contains(order, v) || v >= len(arr) || v < 0 {
			//有重复，证明该序列无效，重建顺序序列
			erri = 3

			neworder = make([]int, 0, len(arr))

			for i := 0; i < len(arr); i++ {
				neworder = append(neworder, i)
			}
			order = neworder
			break
		}
		order[i] = v
	}

	result = make([]T, len(arr))
	for i, v := range order {
		result[i] = arr[v]
	}

	return
}

func MoveItem[T any](arr *[]T, fromIndex, toIndex int) {
	var item = (*arr)[fromIndex]
	Splice(arr, fromIndex, 1)
	Splice(arr, toIndex, 0, item)
}

// splices 包在  Nov 10, 2022 添加了Replace函数, 就不用我们自己的实现了
// v0.0.0-20221110155412-d0897a79cd37, 不过我们为了代码兼容依然保存该代码,直到2.x.x版本.
//
// items to insert at start, delete deleteCount items at start
// See https://github.com/zzwx/splice/blob/main/splice.go
func Splice[T any](source *[]T, start int, deleteCount int, items ...T) (removed []T) {
	if start > len(*source) {
		start = len(*source)
	}
	if start < 0 {
		start = len(*source) + start
	}
	if start < 0 {
		start = 0
	}
	if deleteCount < 0 {
		deleteCount = 0
	}
	if deleteCount > 0 {
		for i := 0; i < deleteCount; i++ {
			if i+start < len(*source) {
				removed = append(removed, (*source)[i+start])
			}
		}
	}
	deleteCount = len(removed) // Adjust to actual delete count
	grow := len(items) - deleteCount
	switch {
	case grow > 0: // So we grow
		*source = append(*source, make([]T, grow)...)
		copy((*source)[start+deleteCount+grow:], (*source)[start+deleteCount:])
	case grow < 0: // So we shrink
		from := start + len(items)
		to := start + deleteCount
		copy((*source)[from:], (*source)[to:])
		*source = (*source)[:len(*source)+grow]
	}
	copy((*source)[start:], items)
	return
}

func GetMapSortedKeySlice[K constraints.Ordered, V any](theMap map[K]V) []K {
	result := make([]K, len(theMap))

	i := 0
	for f := range theMap {
		result[i] = f
		i++
	}
	// 为何 泛型sort比 interface{} sort 快:
	// https://eli.thegreenplace.net/2022/faster-sorting-with-go-generics/

	slices.Sort(result)

	return result
}

// 本作的惯例, 经常使用如下字符串作为配置： s = "e1:v1\ne2:v2",
func CommonSplit(s, e1, e2 string) (ok bool, v1, v2 string) {
	return CommonSplit_strings(s, e1, e2) //经过benchmark，strings比正则快
}

func CommonSplit_strings(s, e1, e2 string) (ok bool, v1, v2 string) {
	s = strings.TrimSuffix(s, "\n")
	lines := strings.SplitN(s, "\n", 2)
	if len(lines) != 2 {
		return
	}

	strs1 := strings.SplitN(lines[0], ":", 2)
	if strs1[0] != e1 {

		return
	}
	v1 = strs1[1]

	strs2 := strings.SplitN(lines[1], ":", 2)
	if strs2[0] != e2 {

		return
	}
	v2 = strs2[1]
	ok = true
	return
}

const commonSplitRegexPattern = `^([^:]+):([^:\n]+)\n([^:]+):([^:\n]+)$`

var commonSplitRegex = regexp.MustCompile(commonSplitRegexPattern)

func CommonSplit_regex(s, e1, e2 string) (ok bool, v1, v2 string) {

	matches := commonSplitRegex.FindAllStringSubmatch(s, -1)
	if len(matches) != 1 {
		return
	}

	match := matches[0]
	if len(match) != 5 {
		return
	}
	if match[1] != e1 || match[3] != e2 {
		return
	}
	v1 = match[2]
	v2 = match[4]
	ok = true
	return
}

// the first part of synonyms is the one to be replaced, the last part of synonyms is the persistent one.
func ReplaceBytesSynonyms(bs []byte, synonyms [][2][]byte) (result []byte) {
	result = bs
	for _, ss := range synonyms {

		result = bytes.Replace(result, ss[0], ss[1], -1)
	}
	return result
}

// same as ReplaceBytesSynonyms
func ReplaceStringsSynonyms(bs string, synonyms [][2]string) (result string) {
	result = bs
	for _, ss := range synonyms {

		result = strings.Replace(result, ss[0], ss[1], -1)
	}
	return result
}
