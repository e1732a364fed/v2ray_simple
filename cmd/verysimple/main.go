package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/profile"
	"go.uber.org/zap"

	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/machine"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	configFileName     string
	useNativeUrlFormat bool
	disableSplice      bool
	startPProf         bool
	startMProf         bool
	gui_mode           bool

	listenURL string //用于命令行模式
	dialURL   string //用于命令行模式

	dialTimeoutSecond int

	runCli func()
	runGui func()

	defaultMachine *machine.M
)

const (
	defaultLogFile = "vs_log"
	defaultConfFn  = "client.toml"
	defaultGeoipFn = "GeoLite2-Country.mmdb"

	willExitStr = "Neither valid proxy settings available, nor cli/apiServer/gui running. Exit now.\n"
)

func init() {
	defaultMachine = machine.New()

	flag.IntVar(&utils.LogLevel, "ll", utils.DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")

	//有时发现在某些情况下，dns查询或者tcp链接的建立很慢，甚至超过8秒, 所以开放自定义超时时间，便于在不同环境下测试
	flag.IntVar(&dialTimeoutSecond, "dt", int(netLayer.DialTimeout/time.Second), "dial timeout, in second")
	flag.BoolVar(&defaultMachine.EnableApiServer, "ea", false, "enable api server")

	flag.BoolVar(&startPProf, "pp", false, "pprof")
	flag.BoolVar(&startMProf, "mp", false, "memory pprof")

	flag.BoolVar(&useNativeUrlFormat, "nu", false, "use the proxy-defined url format, instead of the standard verysimple one.")

	flag.BoolVar(&netLayer.UseReadv, "readv", netLayer.DefaultReadvOption, "toggle the use of 'readv' syscall")

	flag.BoolVar(&disableSplice, "ds", false, "if given, then the app won't use splice.")

	flag.StringVar(&configFileName, "c", defaultConfFn, "config file name")

	flag.StringVar(&listenURL, "L", "", "listen URL, only used when no config file is provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL, only used when no config file is provided.")

	flag.StringVar(&utils.LogOutFileName, "lf", defaultLogFile, "output file for log; If empty, no log file will be used.")

	flag.StringVar(&netLayer.GeoipFileName, "geoip", defaultGeoipFn, "geoip maxmind file name (relative or absolute path)")
	flag.StringVar(&netLayer.GeositeFolder, "geosite", netLayer.DefaultGeositeFolder, "geosite folder name (set it to the relative or absolute path of your geosite/data folder)")
	flag.StringVar(&utils.ExtraSearchPath, "path", "", "search path for mmdb, geosite and other required files")

}

func main() {
	os.Exit(mainFunc())
}

func mainFunc() (result int) {
	defer func() {
		//注意，这个recover代码并不是万能的，有时捕捉不到panic。
		if r := recover(); r != nil {
			if ce := utils.CanLogErr("Captured panic!"); ce != nil {

				stack := debug.Stack()

				stackStr := string(stack)

				ce.Write(
					zap.Any("err:", r),
					zap.String("stacktrace", stackStr),
				)

				log.Println(stackStr) //因为 zap 使用json存储值，所以stack这种多行字符串里的换行符和tab 都被转译了，导致可读性比较差，所以还是要 log单独打印出来，可增强命令行的可读性

			} else {
				log.Println("panic captured!", r, "\n", string(debug.Stack()))
			}

			result = -3

			defaultMachine.Stop()
		}
	}()

	utils.ParseFlags()

	if runExitCommands() {
		return
	} else {

		printVersion(os.Stdout)

	}

	// config params step
	{
		if disableSplice {
			netLayer.SystemCanSplice = false
		}
		if startPProf {
			const pprofFN = "cpu.pprof"
			f, err := os.OpenFile(pprofFN, os.O_CREATE|os.O_RDWR, 0644)

			if err == nil {
				defer f.Close()
				err = pprof.StartCPUProfile(f)
				if err == nil {
					defer pprof.StopCPUProfile()
				} else {
					log.Println("pprof.StartCPUProfile failed", err)

				}
			} else {
				log.Println(pprofFN, "can't be created,", err)
			}

		}
		if startMProf {
			//若不使用 NoShutdownHook, 则 我们ctrl+c退出时不会产生 pprof文件
			p := profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.NoShutdownHook)

			defer p.Stop()
		}

		if useNativeUrlFormat {
			proxy.UrlFormat = proxy.UrlNativeFormat
		}

		netLayer.DialTimeout = time.Duration(dialTimeoutSecond) * time.Second
	}

	var configMode int
	var fallback *httpLayer.ClassicFallback

	var loadConfigErr error

	fpath := utils.GetFilePath(configFileName)
	if !utils.FileExist(fpath) {

		if utils.GivenFlags["c"] == nil {
			log.Printf("No -c provided and default %q doesn't exist", defaultConfFn)
		} else {
			log.Printf("-c provided but %q doesn't exist", configFileName)
		}

		configFileName = ""

	}

	var simpleConf proxy.SimpleConf

	configMode, simpleConf, fallback, loadConfigErr = LoadConfig(configFileName, listenURL, dialURL)

	if loadConfigErr == nil {

		machine.SetupByAppConf(appConf)

	}

	if utils.LogOutFileName == defaultLogFile {

		if strings.Contains(configFileName, "server") {
			utils.LogOutFileName += "_server"
		} else if strings.Contains(configFileName, "client") {
			utils.LogOutFileName += "_client"
		}
	}

	utils.InitLog("Program started")
	defer utils.Info("Program exited")

	utils.Info(versionStr())

	{
		wdir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		if ce := utils.CanLogInfo("Working at"); ce != nil {
			ce.Write(zap.String("dir", wdir))
		}
	}
	if ce := utils.CanLogDebug("All Given Flags"); ce != nil {
		ce.Write(zap.Any("flags", utils.GivenFlagKVs()))
	}

	if loadConfigErr != nil && !IsFlexible(defaultMachine) {

		if ce := utils.CanLogErr(willExitStr); ce != nil {
			ce.Write(zap.Error(loadConfigErr))
		} else {
			log.Print(willExitStr)

		}

		return -1
	}

	//netLayer.PrepareInterfaces()	//发现有时, ipv6不是程序刚运行时就有的, 所以不应默认 预读网卡。主要是 openwrt等设备 在使用 DHCPv6 获取ipv6 等情况时。

	fmt.Printf("Log Level:%d\n", utils.LogLevel)

	if ce := utils.CanLogInfo("Options"); ce != nil {
		ce.Write(
			zap.String("Log Level", utils.LogLevelStr(utils.LogLevel)),
			zap.Bool("UseReadv", netLayer.UseReadv),
		)
	} else {
		fmt.Printf("UseReadv:%t\n", netLayer.UseReadv)
	}

	if fallback != nil {
		defaultMachine.RoutingEnv.Fallback = fallback
	}

	//load inServers and RoutingEnv
	switch configMode {
	case proxy.SimpleMode:
		result, _ = defaultMachine.LoadSimpleServer(simpleConf)
		if result < 0 {
			return result
		}

	case proxy.StandardMode:

		if appConf != nil {
			defaultMachine.DefaultUUID = appConf.DefaultUUID
		}

		//虽然标准模式支持多个Server，目前先只考虑一个
		//多个Server存在的话，则必须要用 tag指定路由; 然后，我们需在预先阶段就判断好tag指定的路由

		if len(standardConf.Listen) < 1 {
			utils.Warn("no listen in config settings")
			break
		}

		defaultMachine.LoadListenConf(standardConf.Listen, false)

		if len(standardConf.Fallbacks) > 0 {
			defaultMachine.ParseFallbacksAtSymbol(standardConf.Fallbacks)
		}
		var myCountryISO_3166 string
		if appConf != nil {
			myCountryISO_3166 = appConf.MyCountryISO_3166
		}

		defaultMachine.RoutingEnv = proxy.LoadEnvFromStandardConf(&standardConf, myCountryISO_3166)

	}

	// load outClients
	switch configMode {
	case proxy.SimpleMode:
		result, defaultMachine.DefaultOutClient = defaultMachine.LoadSimpleClient(simpleConf)
		if result < 0 {
			return result
		}
	case proxy.StandardMode:

		if len(standardConf.Dial) < 1 {
			utils.Warn("no dial in config settings, will add 'direct'")

			defaultMachine.SetDefaultDirectClient()

			break
		}

		defaultMachine.LoadDialConf(standardConf.Dial)

	}

	runPreCommands()

	stopGorouteCaptureSignalChan := make(chan struct{})

	go func() {
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM) //os.Kill cannot be trapped
		select {
		case <-stopGorouteCaptureSignalChan:
			return
		case <-osSignals:
			utils.Info("Program got close signal.")

			defaultMachine.Stop()
			os.Exit(-1)
		}

	}()

	defaultMachine.Start()

	//没可用的listen/dial，而且还无法动态更改配置
	if NoFuture(defaultMachine) {
		utils.Error(willExitStr)
		return -1
	}

	if defaultMachine.EnableApiServer {

		defaultMachine.ApiServerConf = defaultApiServerConf

		var thepass string

		if defaultMachine.AdminPass == "" && appConf != nil {
			if ap := appConf.AdminPass; ap != "" {
				thepass = ap
			}
			defaultMachine.AdminPass = thepass

		}

		defaultMachine.TryRunApiServer()

	}

	if interactive_mode {
		if runCli != nil {
			runCli()

		}

		interactive_mode = false
	}

	if gui_mode {
		if runGui != nil {
			runGui()
			gui_mode = false
			utils.Info("gui mode exited")
		}
	}

	if NothingRunning(defaultMachine) {
		utils.Warn(willExitStr)
		return
	}

	{
		close(stopGorouteCaptureSignalChan)

		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM) //os.Kill cannot be trapped
		<-osSignals

		utils.Info("Program got close signal.")

		defaultMachine.Stop()
	}
	return
}

// 是否可以在运行时动态修改配置。如果没有开启 apiServer 开关 也没有 动态修改配置的功能，则当前模式不灵活，无法动态修改
func IsFlexible(m *machine.M) bool {
	return interactive_mode || gui_mode || m.EnableApiServer
}

func NoFuture(m *machine.M) bool {
	return !m.HasProxyRunning() && !IsFlexible(m)
}

func NothingRunning(m *machine.M) bool {
	return !m.HasProxyRunning() && !(interactive_mode || gui_mode || m.ApiServerRunning)
}
