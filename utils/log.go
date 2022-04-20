package utils

import (
	"flag"
	"fmt"
	"os"

	"github.com/natefinch/lumberjack"
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
	log_off //不支持不打印致命输出。既然致命我们一定要尸检然后查看病因啊

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

	LogOutFileName string

	//在 go test时，我们依然想要查看命令行输出, 但是不希望产生多余文件.
	// 所以只有在main.go 中我们才设置 ShouldLogToFile=true
	ShouldLogToFile bool
)

func init() {

	flag.IntVar(&LogLevel, "ll", DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")

	flag.StringVar(&LogOutFileName, "lf", "vs_log", "output file for log; If empty, no log file will be used.")
}

func LogLevelStrList() (sl []string) {
	sl = make([]string, 0, log_off)
	for i := 0; i < log_off; i++ {
		sl = append(sl, LogLevelStr(i))
	}
	return
}

func LogLevelStr(lvl int) string {
	return zapcore.Level(lvl - 1).String()
}

func getZapLogFileWriteSyncer(fn string) zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   fn,
		MaxSize:    10,
		MaxBackups: 10,
		MaxAge:     30,
		Compress:   false,
	})
}

//为了输出日志保持整齐, 统一使用5字节长度的字符串, 少的加尾缀空格, 多的以 点号 进行缩写。
func levelCapitalStrWith5Chars(l zapcore.Level) string {
	switch l {
	case zapcore.DebugLevel:
		return "DEBUG"
	case zapcore.InfoLevel:
		return "INFO "
	case zapcore.WarnLevel:
		return "WARN "
	case zapcore.ErrorLevel:
		return "ERROR"
	case zapcore.DPanicLevel:
		return "DPAN."
	case zapcore.PanicLevel:
		return "PANIC"
	case zapcore.FatalLevel:
		return "FATAL"
	default:
		return fmt.Sprintf("LEVEL(%d)", l)
	}
}
func capitalLevelEncoderWith5Chars(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(levelCapitalStrWith5Chars(l))
}

//本作大量用到zap打印输出, 所以必须调用InitLog函数来初始化，否则就会闪退
func InitLog() {
	atomicLevel := zap.NewAtomicLevel()
	atomicLevel.SetLevel(zapcore.Level(LogLevel - 1))

	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(zapcore.EncoderConfig{
		MessageKey:  "M",
		LevelKey:    "L",
		TimeKey:     "T",
		FunctionKey: "func",
		EncodeLevel: zapcore.CapitalColorLevelEncoder,
		EncodeTime:  zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
	}), zapcore.AddSync(os.Stdout), atomicLevel)

	if ShouldLogToFile && LogOutFileName != "" {
		jsonConf := zap.NewProductionEncoderConfig()
		jsonConf.EncodeTime = zapcore.TimeEncoderOfLayout("060102 150405.000") //用一种比较简短的方式输出时间,年月日 时分秒.毫秒。 年分只需输出后两位数字即可, 不管Y2K问题, 80年后要是还没实现网络自由那这个世界完蛋了.
		jsonConf.LevelKey = "L"
		jsonConf.TimeKey = "T"
		jsonConf.MessageKey = "M"
		jsonConf.EncodeLevel = capitalLevelEncoderWith5Chars
		jsonCore := zapcore.NewCore(zapcore.NewJSONEncoder(jsonConf), getZapLogFileWriteSyncer(LogOutFileName), atomicLevel)

		ZapLogger = zap.New(zapcore.NewTee(consoleCore, jsonCore))

	} else {
		ZapLogger = zap.New(consoleCore)

	}

	//zap.NewDevelopmentEncoderConfig()
	ZapLogger.Info("zap log init complete.")
}

func CanLogLevel(l int, msg string) *zapcore.CheckedEntry {
	return ZapLogger.Check(zapcore.Level(l-1), msg)

}

func CanLogErr(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_error || ZapLogger == nil {
		return nil
	}
	return ZapLogger.Check(zap.ErrorLevel, msg)

}

func CanLogInfo(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_info || ZapLogger == nil {
		return nil
	}
	return ZapLogger.Check(zap.InfoLevel, msg)

}
func CanLogWarn(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_warning || ZapLogger == nil {
		return nil
	}
	return ZapLogger.Check(zap.WarnLevel, msg)

}
func CanLogDebug(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_debug || ZapLogger == nil {
		return nil
	}
	return ZapLogger.Check(zap.DebugLevel, msg)

}
func CanLogFatal(msg string) *zapcore.CheckedEntry {
	if LogLevel > Log_fatal || ZapLogger == nil {
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
