//go:build !noquic

package main

import _ "github.com/e1732a364fed/v2ray_simple/advLayer/quic"

// 如果不引用 quic，go build 编译出的可执行文件 的大小 可以减小 2MB 。
