// Package utils provides utils that needed by all sub-packages in verysimle
package utils

//具体实现见 readv_*.go; 用 GetReadVReader() 函数来获取本平台的对应实现。
type MultiReader interface {
	Init([][]byte)
	Read(fd uintptr) int32
	Clear()
}
