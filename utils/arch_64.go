//go:build amd64 || arm64 || mips64 || mips64le || ppc64 || ppc64le || riscv64 || s390x || wasm

package utils

import (
	"sync/atomic"
)

// Use atomic.AddUint64 at 64bit arch, use sync.mutex at 32bit arch
func AtomicAddUint64(u64 *uint64, delta uint64) {
	atomic.AddUint64(u64, delta)
}
