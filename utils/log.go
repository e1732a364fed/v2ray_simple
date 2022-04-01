// Package utils provides utilities that is used in all sub-packages in verysimple
package utils

import (
	"flag"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	Log_debug = iota
	Log_info
	Log_warning
	Log_error //error一般用于输出一些 连接错误或者客户端协议错误之类的, 但不致命
	Log_fatal
	//Log_off	//不支持不打印致命输出。既然致命我们一定要尸检然后查看病因啊

	DefaultLL = Log_info
)

// LogLevel 值越小越唠叨, 废话越多，值越大打印的越少，见log_开头的常量;
// 默认是 info级别.因为还在开发中，所以默认级别高一些有好处，方便排错
var (
	LogLevel  int
	ZapLogger *zap.Logger
)

func init() {
	//我们的loglevel就是zap的loglevel+1

	flag.IntVar(&LogLevel, "ll", DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")
}

func InitLog() {
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zapcore.Level(LogLevel - 1))

	var writes = []zapcore.WriteSyncer{zapcore.AddSync(os.Stdout)}

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey:  "msg",
		LevelKey:    "level",
		TimeKey:     "time",
		FunctionKey: "func",
		//EncodeTime:  zapcore.ISO8601TimeEncoder,
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
		EncodeTime:  zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeName:  zapcore.FullNameEncoder,
		LineEnding:  zapcore.DefaultLineEnding,
	}), zapcore.NewMultiWriteSyncer(writes...), atomicLevel)

	//zap.NewDevelopmentEncoderConfig()
	ZapLogger = zap.New(core)
	ZapLogger.Info("log 初始化成功")
}

func CanLogLevel(l int, msg string) *zapcore.CheckedEntry {
	return ZapLogger.Check(zapcore.Level(l-1), msg)

}

func canLogLevel(l zapcore.Level, msg string) *zapcore.CheckedEntry {
	return ZapLogger.Check(l, msg)

}

func CanLogErr(msg string) *zapcore.CheckedEntry {
	return canLogLevel(zap.ErrorLevel, msg)

}

func CanLogInfo(msg string) *zapcore.CheckedEntry {
	return canLogLevel(zap.InfoLevel, msg)

}
func CanLogWarn(msg string) *zapcore.CheckedEntry {
	return canLogLevel(zap.WarnLevel, msg)

}
func CanLogDebug(msg string) *zapcore.CheckedEntry {
	return canLogLevel(zap.DebugLevel, msg)

}
