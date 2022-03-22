// Package utils provides utils that needed by all sub-packages in verysimle
package utils

import "flag"

const (
	Log_debug = iota
	Log_info
	Log_warning
	Log_error
	Log_fatal
	//Log_off	//不支持不打印致命输出。既然致命我们一定要尸检然后查看病因啊
)

// LogLevel 值越小越唠叨, 废话越多，值越大打印的越少，见log_开头的常量;
// 默认是 info级别.因为还在开发中，所以默认级别高一些有好处，方便排错
var LogLevel int

func init() {
	flag.IntVar(&LogLevel, "ll", Log_info, "log level,0=debug, 1=info, 2=warning, 3=error, 4=fatal")
}

//return LogLevel <= l
func CanLogLevel(l int) bool {
	return LogLevel <= l

}

func CanLogErr() bool {
	return LogLevel <= Log_error

}

func CanLogInfo() bool {
	return LogLevel <= Log_info

}
func CanLogWarn() bool {
	return LogLevel <= Log_warning

}
func CanLogDebug() bool {
	return LogLevel == 0

}
