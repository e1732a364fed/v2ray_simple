package utils_test

import (
	"testing"

	"github.com/e1732a364fed/v2ray_simple/utils"

	"gonum.org/v1/gonum/stat/combin"
)

func TestCommonSpit(t *testing.T) {
	realv1 := "v1"
	realv2 := "v2"
	s := "e1:" + realv1 + "\ne2:" + realv2
	ok, v1, v2 := utils.CommonSplit(s, "e1", "e2")

	if !ok || realv1 != v1 || realv2 != v2 {
		t.FailNow()
	}
}

var x = []string{"AA", "BB", "CC", "DD"}
var y = []int{1, 2, 3, 4}

func TestSplice(t *testing.T) {
	t.Log(utils.TrimSlice(utils.CloneSlice(x), 0))
	t.Log(utils.TrimSlice(utils.CloneSlice(x), 1))
	t.Log(utils.TrimSlice(utils.CloneSlice(x), 2))
	t.Log(utils.TrimSlice(utils.CloneSlice(x), 3))
	t.Log(utils.TrimSlice(utils.CloneSlice(y), 0))
	t.Log(utils.TrimSlice(utils.CloneSlice(y), 1))
	t.Log(utils.TrimSlice(utils.CloneSlice(y), 2))
	t.Log(utils.TrimSlice(utils.CloneSlice(y), 3))
}

/*
BenchmarkAllSubSets_4-8                       	  969097	      1198 ns/op
BenchmarkAllSubSets_3-8                       	 2340783	       514.6 ns/op
BenchmarkAllSubSets_2-8                       	 5716852	       210.4 ns/op
BenchmarkAllSubSets_Int_4-8                   	 1472281	       821.6 ns/op
BenchmarkAllSubSets_Int_3-8                   	 3108220	       384.3 ns/op
BenchmarkAllSubSets_Int_2-8                   	 6744888	       177.6 ns/op
BenchmarkAllSubSets_improve1_Int_4-8          	 3213049	       370.2 ns/op
BenchmarkAllSubSets_improve1_Int_3-8          	 6796364	       174.2 ns/op
BenchmarkAllSubSets_improve1_Int_2-8          	15821853	        76.57 ns/op
BenchmarkAllSubSets_gonum_Int_4-8             	 3169869	       377.6 ns/op
BenchmarkAllSubSets_gonum_Int_3-8             	 6271921	       190.2 ns/op
BenchmarkAllSubSets_gonum_Int_2-8             	13109756	        91.19 ns/op
BenchmarkAllSubSets_gonum_generator_Int_4-8   	 7368919	       163.3 ns/op
BenchmarkAllSubSets_gonum_generator_Int_3-8   	11627896	       102.5 ns/op
BenchmarkAllSubSets_gonum_generator_Int_2-8   	19748900	        60.58 ns/op


总之最大开销就是在内存分配，果然还是用 combin.NewCombinationGenerator 最快，
其次是我的improve方法, 然后是gonum普通方法, 最差的是 golang-combinations 的原方法
*/

func TestAllSubSets(t *testing.T) {
	t.Log(utils.AllSubSets(x))
}

func BenchmarkAllSubSets_4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(x)
	}
}

func BenchmarkAllSubSets_3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(x[:3])
	}
}
func BenchmarkAllSubSets_2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(x[:2])
	}
}
func BenchmarkAllSubSets_Int_4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(y)
	}
}

func BenchmarkAllSubSets_Int_3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(y[:3])
	}
}
func BenchmarkAllSubSets_Int_2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets(y[:2])
	}
}

func BenchmarkAllSubSets_improve1_Int_4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets_improve1(y)
	}
}

func BenchmarkAllSubSets_improve1_Int_3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets_improve1(y[:3])
	}
}
func BenchmarkAllSubSets_improve1_Int_2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		utils.AllSubSets_improve1(y[:2])
	}
}

//https://pkg.go.dev/gonum.org/v1/gonum@v0.11.0/stat/combin

func BenchmarkAllSubSets_gonum_Int_4(b *testing.B) {
	for i := 0; i < b.N; i++ {
		combin.Combinations(4, 4)
		combin.Combinations(4, 3)
		combin.Combinations(4, 2)
		combin.Combinations(4, 1)
	}
}

func BenchmarkAllSubSets_gonum_Int_3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		combin.Combinations(3, 3)
		combin.Combinations(3, 2)
		combin.Combinations(3, 1)
	}
}

func BenchmarkAllSubSets_gonum_Int_2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		combin.Combinations(2, 2)
		combin.Combinations(2, 1)
	}
}

func BenchmarkAllSubSets_gonum_generator_Int_4(b *testing.B) {
	for i := 0; i < b.N; i++ {

		combin.Combinations(4, 4)

		cg := combin.NewCombinationGenerator(4, 3)
		buf := make([]int, 3)
		for cg.Next() {
			cg.Combination(buf)

		}
		cg = combin.NewCombinationGenerator(4, 2)
		buf = buf[:2]
		for cg.Next() {
			cg.Combination(buf)

		}
		cg = combin.NewCombinationGenerator(4, 1)
		buf = buf[:1]
		for cg.Next() {
			cg.Combination(buf)

		}
	}
}

func BenchmarkAllSubSets_gonum_generator_Int_3(b *testing.B) {
	for i := 0; i < b.N; i++ {

		combin.Combinations(3, 3)

		cg := combin.NewCombinationGenerator(3, 2)
		buf := make([]int, 2)
		for cg.Next() {
			cg.Combination(buf)

		}
		cg = combin.NewCombinationGenerator(3, 1)
		buf = buf[:1]
		for cg.Next() {
			cg.Combination(buf)

		}
	}
}

func BenchmarkAllSubSets_gonum_generator_Int_2(b *testing.B) {
	for i := 0; i < b.N; i++ {

		combin.Combinations(2, 2)

		cg := combin.NewCombinationGenerator(2, 1)
		buf := make([]int, 1)
		for cg.Next() {
			cg.Combination(buf)

		}

	}
}
