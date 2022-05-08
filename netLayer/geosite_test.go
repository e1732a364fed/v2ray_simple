package netLayer

import "testing"

func TestGeosite(t *testing.T) {
	//DownloadCommunity_DomainListFiles()
	// 这个需要从github下载，所以如果你的网无法访问gituhb的话是会失败的, 所以就不在代码里下载了,否则再怨我说go test不通过. 我已经试了好使.

	err := LoadGeositeFiles()
	if err != nil {
		return //本测试需要在下载好geosite文件后再运行，所以没下载时运行导致找不到文件的也不能认为就未通过test
	}
	t.Log(len(GeositeListMap))

	//inclusionCount := 0
	var typeCount map[string]int = make(map[string]int)
	//for n, list := range GeositeListMap {
	for _, list := range GeositeListMap {
		//if len(list.Inclusion) > 0 {
		//t.Log("==========================", list.Inclusion)
		//	inclusionCount++
		//}
		for _, d := range list.Domains {
			//t.Log(n, d.Type, d.Value, d.Attrs)
			typeCount[d.Type] = typeCount[d.Type] + 1
		}
	}

	//t.Log("inclusionCount", inclusionCount)

	for n, c := range typeCount {
		t.Log(n, c)
	}

	/*
		20220404064336 的测试:

			inclusionCount 79
			regexp 103
			domain 49552
			full 1931
	*/
}
