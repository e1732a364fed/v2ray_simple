package utils

import (
	"regexp"
	"strings"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

//Combinatorics ////////////////////////////////////////////////////////////////

//func AllSubSets edited from https://github.com/mxschmitt/golang-combinations with MIT License
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

//AllSubSets 测速有点慢, 我改进一下内存分配,可加速一倍多
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

	//实际上 golang.org/x/exp/slices 的 Clone 函数也可以, 不过我还是觉得我自己的好理解一些
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

	//实际上 golang.org/x/exp/slices 的 Delete 函数也可以, 不过我还是觉得我自己的好理解一些
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

//本作的惯例, 经常使用如下字符串作为配置： s = "e1:v1\ne2:v2",
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
