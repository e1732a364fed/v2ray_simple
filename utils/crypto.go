package utils

import "runtime"

//有些系统对aes支持不好，有些支持好。SystemAutoUseAes若为true，则说明支持很好，使用aes作为加密算法速度最佳。
const SystemAutoUseAes = runtime.GOARCH == "amd64" || runtime.GOARCH == "s390x" || runtime.GOARCH == "arm64"
