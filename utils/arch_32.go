//go:build 386 || arm || mips || mipsle

package utils

import (
	"sync"
)

var uint64Mutex sync.Mutex

// Use AddUint64 at 64bit arch, use sync.mutex at 32bit arch
func AtomicAddUint64(u64 *uint64, delta uint64) {
	uint64Mutex.Lock()
	*u64 = *u64 + delta
	uint64Mutex.Unlock()

}
