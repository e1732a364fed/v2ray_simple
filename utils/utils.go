// Package utils provides utils that needed by all packages of verysimle
package utils

//具体实现见 readv_*.go
type MultiReader interface {
	Init([][]byte)
	Read(fd uintptr) int32
	Clear()
}
