package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"syscall"

	"github.com/pkg/profile"
	"go.uber.org/zap"

	vs "github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

var (
	configFileName string
	startPProf     bool
	startMProf     bool
	listenURL      string //用于命令行模式
	dialURL        string //用于命令行模式
	//jsonMode       int

	standardConf proxy.StandardConf
	simpleConf   proxy.SimpleConf

	allServers = make([]proxy.Server, 0, 8) //储存除tproxy之外 所有运行的 inServer
	allClients = make([]proxy.Client, 0, 8)

	tproxyList []*tproxy.Machine //储存所有 tproxy的监听.(一般就一个, 但不排除极特殊情况)

	listenCloserList []io.Closer //储存除tproxy之外 所有运行的 inServer 的 Listener 的 Closer

	defaultOutClient proxy.Client

	routingEnv proxy.RoutingEnv
)

const (
	defaultLogFile = "vs_log"
	defaultConfFn  = "client.toml"
	defaultGeoipFn = "GeoLite2-Country.mmdb"

	willExitStr = "Neither valid proxy settings available, nor cli or apiServer running. Exit now.\n"
)

func init() {
	routingEnv.ClientsTagMap = make(map[string]proxy.Client)

	flag.StringVar(&configFileName, "c", defaultConfFn, "config file name")
	flag.BoolVar(&startPProf, "pp", false, "pprof")
	flag.BoolVar(&startMProf, "mp", false, "memory pprof")
	//flag.IntVar(&jsonMode, "jm", 0, "json mode, 0:verysimple mode; 1: v2ray mode(not implemented yet)")

	flag.StringVar(&listenURL, "L", "", "listen URL, only used when no config file is provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL, only used when no config file is provided.")

	//other packages

	flag.IntVar(&utils.LogLevel, "ll", utils.DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")

	flag.StringVar(&utils.LogOutFileName, "lf", defaultLogFile, "output file for log; If empty, no log file will be used.")

	flag.BoolVar(&netLayer.UseReadv, "readv", netLayer.DefaultReadvOption, "toggle the use of 'readv' syscall")

	flag.StringVar(&netLayer.GeoipFileName, "geoip", defaultGeoipFn, "geoip maxmind file name")
	flag.StringVar(&utils.ExtraSearchPath, "path", "", "search path for mmdb, geosite and other required files")

}

//我们 在程序关闭时, 主动Close, Stop
func cleanup() {

	for _, ser := range allServers {
		if ser != nil {
			ser.Stop()
		}
	}

	for _, listener := range listenCloserList {
		if listener != nil {
			listener.Close()
		}
	}

	if len(tproxyList) > 0 {
		log.Println("closing tproxies")
		for _, tm := range tproxyList {
			if tm != nil {
				tm.Stop()
			}
		}
	}

}

func main() {
	os.Exit(mainFunc())
}

func mainFunc() (result int) {
	defer func() {
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

			cleanup()
		}
	}()

	utils.ParseFlags()

	if runExitCommands() {
		return
	} else {
		printVersion()

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

	var mode int
	var mainFallback *httpLayer.ClassicFallback

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

	standardConf, simpleConf, mode, mainFallback, loadConfigErr = proxy.LoadConfig(configFileName, listenURL, dialURL, 0)

	if loadConfigErr == nil {

		if appConf := standardConf.App; appConf != nil {

			if appConf.LogFile != nil && utils.GivenFlags["lf"] == nil {
				utils.LogOutFileName = *appConf.LogFile

			}

			if appConf.LogLevel != nil && utils.GivenFlags["ll"] == nil {
				utils.LogLevel = *appConf.LogLevel

			}
			if appConf.NoReadV && utils.GivenFlags["readv"] == nil {
				netLayer.UseReadv = false
			}

		}

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

	if loadConfigErr != nil && !isFlexible() {

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

	var defaultInServer proxy.Server
	var Default_uuid string

	if mainFallback != nil {
		routingEnv.MainFallback = mainFallback
	}

	var tproxyConfs []*proxy.ListenConf

	//load inServers and RoutingEnv
	switch mode {
	case proxy.SimpleMode:
		result, defaultInServer = loadSimpleServer()
		if result < 0 {
			return result
		}
	case proxy.StandardMode:

		if appConf := standardConf.App; appConf != nil {
			Default_uuid = appConf.DefaultUUID
		}

		//虽然标准模式支持多个Server，目前先只考虑一个
		//多个Server存在的话，则必须要用 tag指定路由; 然后，我们需在预先阶段就判断好tag指定的路由

		if len(standardConf.Listen) < 1 {
			utils.Warn("no listen in config settings")
			break
		}

		for _, serverConf := range standardConf.Listen {
			thisConf := serverConf
			if thisConf.Protocol == "tproxy" {
				tproxyConfs = append(tproxyConfs, thisConf)

				continue
			}

			if thisConf.Uuid == "" && Default_uuid != "" {
				thisConf.Uuid = Default_uuid
			}

			thisServer, err := proxy.NewServer(thisConf)
			if err != nil {
				if ce := utils.CanLogErr("can not create local server:"); ce != nil {
					ce.Write(zap.Error(err))
				}
				continue
			}

			allServers = append(allServers, thisServer)
		}

		//将@前缀的 回落dest配置 替换成 实际的 地址。

		if len(standardConf.Fallbacks) > 0 {
			for _, fbConf := range standardConf.Fallbacks {
				if fbConf.Dest == nil {
					continue
				}
				if deststr, ok := fbConf.Dest.(string); ok && strings.HasPrefix(deststr, "@") {
					for _, s := range allServers {
						if s.GetTag() == deststr[1:] {
							log.Println("got tag fallback dest, will set to ", s.AddrStr())
							fbConf.Dest = s.AddrStr()
						}
					}

				}

			}
		}

		routingEnv = proxy.LoadEnvFromStandardConf(&standardConf)

	}

	// load outClients
	switch mode {
	case proxy.SimpleMode:
		result, defaultOutClient = loadSimpleClient()
		if result < 0 {
			return result
		}
	case proxy.StandardMode:

		if len(standardConf.Dial) < 1 {
			utils.Warn("no dial in config settings, will add 'direct'")

			allClients = append(allClients, vs.DirectClient)
			defaultOutClient = vs.DirectClient

			routingEnv.SetClient("direct", vs.DirectClient)

			break
		}

		hotLoadDialConfForRuntime(Default_uuid, standardConf.Dial)

	}

	runPreCommands()

	if (defaultOutClient != nil) && (defaultInServer != nil || len(allServers) > 0 || len(tproxyConfs) > 0) {

		if mode == proxy.SimpleMode {
			lis := vs.ListenSer(defaultInServer, defaultOutClient, &routingEnv)
			if lis != nil {
				listenCloserList = append(listenCloserList, lis)
			}
		} else {
			for _, inServer := range allServers {
				lis := vs.ListenSer(inServer, defaultOutClient, &routingEnv)

				if lis != nil {
					listenCloserList = append(listenCloserList, lis)
				}
			}

			if len(tproxyConfs) > 0 {

				if len(tproxyConfs) == 1 {
					conf := tproxyConfs[0]
					if thing := conf.Extra["auto_iptables"]; thing != nil {
						if auto, ok := thing.(bool); ok && auto {
							tproxy.SetIPTablesByPort(conf.Port)

							defer func() {
								tproxy.CleanupIPTables()
							}()
						}
					}

				}

				for _, thisConf := range tproxyConfs {
					enableSniff := false
					if thisConf.SniffConf != nil {
						enableSniff = thisConf.SniffConf.Enable
					}
					lc := proxy.LesserConf{
						Addr:        thisConf.GetAddrStrForListenOrDial(),
						Tag:         thisConf.Tag,
						UseSniffing: enableSniff,
						Fullcone:    thisConf.Fullcone,
					}
					tm := vs.ListenTproxy(lc, defaultOutClient, &routingEnv)
					if tm != nil {
						tproxyList = append(tproxyList, tm)
					}

				}

			} //if len(tproxyConfs) > 0 {

		} //if mode == proxy.SimpleMode {

	} //if (defaultOutClient != nil) && (defaultInServer != nil || len(allServers) > 0 || len(tproxyConfs) > 0) {

	//没可用的listen/dial，而且还无法动态更改配置
	if noFuture() {
		utils.Error(willExitStr)
		return -1
	}

	if enableApiServer {
		tryRunApiServer()

	}

	if interactive_mode {
		runCli()

		interactive_mode = false
	}

	if nothingRunning() {
		utils.Warn(willExitStr)
		return
	}

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM) //os.Kill cannot be trapped
		<-osSignals

		utils.Info("Program got close signal.")

		cleanup()
	}
	return
}

func hasProxyRunning() bool {
	return len(listenCloserList) > 0 || len(tproxyList) > 0
}

//是否可以在运行时动态修改配置。如果没有开启 apiServer 开关 也没有 动态修改配置的功能，则当前模式不灵活，无法动态修改
func isFlexible() bool {
	return interactive_mode || enableApiServer
}

func noFuture() bool {
	return !hasProxyRunning() && !isFlexible()
}

func nothingRunning() bool {
	return !hasProxyRunning() && !(interactive_mode || apiServerRunning)
}
