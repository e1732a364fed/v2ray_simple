//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package utils

func GetRandomWord() string {
	// 在非 unix/linux 系统下， 不必使用 babble包，因为该包太大了，占用空间.
	// unix下我们也没使用babble包，因为代码太简单，直接复制过来了。

	return GenerateRandomString()
}
