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

	listenerArray []io.Closer //储存除tproxy之外 所有运行的 inServer 的 Listener

	defaultOutClient proxy.Client

	routingEnv proxy.RoutingEnv
)

const (
	defaultLogFile = "vs_log"
	defaultConfFn  = "client.toml"
	defaultGeoipFn = "GeoLite2-Country.mmdb"
)

func initRouteEnv() {
	routingEnv.ClientsTagMap = make(map[string]proxy.Client)

}

func init() {
	initRouteEnv()

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

}

func cleanup() {
	//在程序ctrl+C关闭时, 会主动Close所有的监听端口. 主要是被报告windows有时退出程序之后, 端口还是处于占用状态.
	// 用下面代码以试图解决端口占用问题.

	for _, listener := range listenerArray {
		if listener != nil {
			listener.Close()
		}
	}

	for _, tm := range tproxyList {
		if tm != nil {
			tm.Stop()
		}
	}
}

func main() {
	os.Exit(mainFunc())
}

func mainFunc() (result int) {
	defer func() {
		if r := recover(); r != nil {
			if ce := utils.CanLogFatal("Captured panic!"); ce != nil {

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
		f, err := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0644)

		if err == nil {
			defer f.Close()
			err = pprof.StartCPUProfile(f)
			if err == nil {
				defer pprof.StopCPUProfile()
			}
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

	//如果-c参数没给出，那么默认的值 很可能没有对应的文件
	fpath := utils.GetFilePath(configFileName)
	if !utils.FileExist(fpath) {

		if utils.GivenFlags["c"] == nil {
			log.Printf("No -c provided and no %q provided", defaultConfFn)
		} else {
			log.Printf("No %q provided", configFileName)
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

	if loadConfigErr != nil && !isFlexible() {

		const exitStr = "no config exist, and no api server or interactive cli enabled, exiting..."

		if ce := utils.CanLogErr(exitStr); ce != nil {
			ce.Write()
		} else {
			log.Printf(exitStr)

		}

		return -1
	}

	netLayer.Prepare()

	fmt.Printf("Log Level:%d\n", utils.LogLevel)

	if ce := utils.CanLogInfo("Options"); ce != nil {

		ce.Write(
			zap.String("Log Level", utils.LogLevelStr(utils.LogLevel)),
			zap.Bool("UseReadv", netLayer.UseReadv),
			zap.Bool("tls_lazy_encrypt", vs.Tls_lazy_encrypt),
		)

	} else {

		fmt.Printf("UseReadv:%t\n", netLayer.UseReadv)
		fmt.Printf("tls_lazy_encrypt:%t\n", vs.Tls_lazy_encrypt)

	}

	runPreCommands()

	var defaultInServer proxy.Server
	var Default_uuid string

	if mainFallback != nil {
		routingEnv.MainFallback = mainFallback
	}

	var tproxyConfs []*proxy.ListenConf

	//load inServers and RoutingEnv
	switch mode {
	case proxy.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		defaultInServer, hase, eie = proxy.ServerFromURL(simpleConf.ListenUrl)
		if hase {
			if ce := utils.CanLogErr("can not create local server"); ce != nil {
				ce.Write(zap.Error(eie))
			}
			return -1
		}

		if !defaultInServer.CantRoute() && simpleConf.Route != nil {

			netLayer.LoadMaxmindGeoipFile("")

			//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
			routingEnv.RoutePolicy = netLayer.NewRoutePolicy()
			if simpleConf.MyCountryISO_3166 != "" {
				routingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(simpleConf.MyCountryISO_3166))

			}
		}
	case proxy.StandardMode:

		routingEnv, Default_uuid = proxy.LoadEnvFromStandardConf(&standardConf)

		initRouteEnv()

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

	}

	// load outClients
	switch mode {
	case proxy.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		defaultOutClient, hase, eie = proxy.ClientFromURL(simpleConf.DialUrl)
		if hase {
			if ce := utils.CanLogErr("can not create remote client"); ce != nil {
				ce.Write(zap.Error(eie))
			}
			return -1
		}
	case proxy.StandardMode:

		if len(standardConf.Dial) < 1 {
			utils.Warn("no dial in config settings")
			break
		}

		for _, thisConf := range standardConf.Dial {
			if thisConf.Uuid == "" && Default_uuid != "" {
				thisConf.Uuid = Default_uuid
			}

			thisClient, err := proxy.NewClient(thisConf)
			if err != nil {
				if ce := utils.CanLogErr("can not create remote client: "); ce != nil {
					ce.Write(zap.Error(err))
				}
				continue
			}
			allClients = append(allClients, thisClient)

			if tag := thisClient.GetTag(); tag != "" {
				routingEnv.ClientsTagMap[tag] = thisClient
			}
		}

		if len(allClients) > 0 {
			defaultOutClient = allClients[0]

		} else {
			defaultOutClient = vs.DirectClient
		}

	}

	configFileQualifiedToRun := false

	if (defaultOutClient != nil) && (defaultInServer != nil || len(allServers) > 0 || len(tproxyConfs) > 0) {
		configFileQualifiedToRun = true

		if mode == proxy.SimpleMode {
			lis := vs.ListenSer(defaultInServer, defaultOutClient, &routingEnv)
			if lis != nil {
				listenerArray = append(listenerArray, lis)
			}
		} else {
			for _, inServer := range allServers {
				lis := vs.ListenSer(inServer, defaultOutClient, &routingEnv)

				if lis != nil {
					listenerArray = append(listenerArray, lis)
				}
			}

			if len(tproxyConfs) > 0 {
				autoIptable := false
				if len(tproxyConfs) == 1 {
					conf := tproxyConfs[0]
					if thing := conf.Extra["auto_iptables"]; thing != nil {
						if auto, ok := thing.(bool); ok && auto {
							autoIptable = true
						}
					}

					if autoIptable {
						tproxy.SetIPTablesByPort(conf.Port)

						defer func() {
							tproxy.CleanupIPTables()
						}()
					}
				}

				for _, thisConf := range tproxyConfs {
					tm := vs.ListenTproxy(thisConf.GetAddrStrForListenOrDial(), defaultOutClient, routingEnv.RoutePolicy)
					if tm != nil {
						tproxyList = append(tproxyList, tm)
					}

				}

				//如果在非linux系统 上，inServer 仅设置了tproxy，则会遇到下面情况
				if len(tproxyList) == 0 {
					if !(defaultInServer != nil || len(allServers) > 0) {
						configFileQualifiedToRun = false
					}
				}
			}

		}

	}
	//没可用的listen/dial，而且还无法动态更改配置
	if !configFileQualifiedToRun && !isFlexible() {
		utils.Error("No valid proxy settings available, nor cli or apiServer feature enabled, exit now.")
		return -1
	}

	if enableApiServer {
		go checkConfigAndTryRunApiServer()

	}

	if interactive_mode {
		runCli()
	}

	{
		osSignals := make(chan os.Signal, 1)
		signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
		<-osSignals

		utils.Info("Program got close signal.")

		cleanup()
	}
	return
}
