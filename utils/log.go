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
	log_dpanic
	log_panic
	Log_fatal
	//Log_off	//不支持不打印致命输出。既然致命我们一定要尸检然后查看病因啊

	DefaultLL = Log_warning
)

// LogLevel 值越小越唠叨, 废话越多，值越大打印的越少，见log_开头的常量;
//
// 默认是 info级别.因为还在开发中，所以默认级别高一些有好处，方便排错
//
//我们的loglevel具体值 与 zap的 loglevel+1 的含义等价
var (
	LogLevel  int
	ZapLogger *zap.Logger
)

func init() {

	flag.IntVar(&LogLevel, "ll", DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")
}

//本作大量用到zap打印输出, 所以必须调用InitLog函数来初始化，否则就会闪退
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
	ZapLogger.Info("zap log init complete.")
}

func CanLogLevel(l int, msg string) *zapcore.CheckedEntry {
	return ZapLogger.Check(zapcore.Level(l-1), msg)

}

func CanLogErr(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_error {
		return nil
	}
	return ZapLogger.Check(zap.ErrorLevel, msg)

}

func CanLogInfo(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_info {
		return nil
	}
	return ZapLogger.Check(zap.InfoLevel, msg)

}
func CanLogWarn(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_warning {
		return nil
	}
	return ZapLogger.Check(zap.WarnLevel, msg)

}
func CanLogDebug(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_debug {
		return nil
	}
	return ZapLogger.Check(zap.DebugLevel, msg)

}
func CanLogFatal(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_fatal {
		return nil
	}
	return ZapLogger.Check(zap.FatalLevel, msg)

}

func Debug(msg string) {
	ZapLogger.Debug(msg)
}
func Info(msg string) {
	ZapLogger.Info(msg)
}
func Warn(msg string) {
	ZapLogger.Warn(msg)
}
func Error(msg string) {
	ZapLogger.Error(msg)
}
func Fatal(msg string) {
	ZapLogger.Fatal(msg)
}
