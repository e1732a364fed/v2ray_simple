package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"

	vs "github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/httpLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/netLayer/tproxy"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/pkg/profile"
	"go.uber.org/zap"
)

var (
	configFileName string
	startPProf     bool
	startMProf     bool
	listenURL      string //用于命令行模式
	dialURL        string //用于命令行模式
	jsonMode       int

	standardConf proxy.StandardConf
	simpleConf   proxy.SimpleConf

	AllServers = make([]proxy.Server, 0, 8)
	AllClients = make([]proxy.Client, 0, 8)

	DefaultOutClient proxy.Client

	TproxyList []*tproxy.Machine

	ListenerArray []net.Listener

	RoutingEnv proxy.RoutingEnv
)

func init() {
	flag.StringVar(&configFileName, "c", "client.toml", "config file name")
	flag.BoolVar(&startPProf, "pp", false, "pprof")
	flag.BoolVar(&startMProf, "mp", false, "memory pprof")
	flag.IntVar(&jsonMode, "jm", 0, "json mode, 0:verysimple mode; 1: v2ray mode(not implemented yet)")

	flag.StringVar(&listenURL, "L", "", "listen URL (i.e. the listen part in config file), only enbled when config file is not provided.")
	flag.StringVar(&dialURL, "D", "", "dial URL (i.e. the dial part in config file), only enbled when config file is not provided.")

	//other packages

	flag.IntVar(&utils.LogLevel, "ll", utils.DefaultLL, "log level,0=debug, 1=info, 2=warning, 3=error, 4=dpanic, 5=panic, 6=fatal")

	flag.StringVar(&utils.LogOutFileName, "lf", "vs_log", "output file for log; If empty, no log file will be used.")

	flag.BoolVar(&netLayer.UseReadv, "readv", netLayer.DefaultReadvOption, "toggle the use of 'readv' syscall")

	flag.StringVar(&netLayer.GeoipFileName, "geoip", "GeoLite2-Country.mmdb", "geoip maxmind file name")

}

func main() {
	os.Exit(mainFunc())
}

func mainFunc() (result int) {
	defer func() {
		if r := recover(); r != nil {
			if ce := utils.CanLogFatal("Captured panic!"); ce != nil {
				ce.Write(zap.Any("err:", r))
			} else {
				log.Fatalln("panic captured!", r)
			}

			result = -3

			cleanup()
		}
	}()

	flag.Parse()

	if cmdPrintVer {
		printVersion_simple()
		//根据 cmdPrintVer 的定义, 我们直接退出
		return
	} else {
		printVersion()

	}

	if !utils.IsFlagGiven("lf") {
		if strings.Contains(configFileName, "server") {
			utils.LogOutFileName += "_server"
		} else if strings.Contains(configFileName, "client") {
			utils.LogOutFileName += "_client"
		}
	}

	if utils.LogOutFileName != "" {
		utils.ShouldLogToFile = true
	}

	utils.InitLog()

	if startPProf {
		f, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

	}
	if startMProf {
		//若不使用 NoShutdownHook, 则 我们ctrl+c退出时不会产生 pprof文件
		p := profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.NoShutdownHook)

		defer p.Stop()
	}

	utils.Info("Program started")
	defer utils.Info("Program exited")

	var err error
	var mode int
	var mainFallback *httpLayer.ClassicFallback

	standardConf, simpleConf, mode, mainFallback, err = proxy.LoadConfig(configFileName, listenURL, dialURL)

	if err != nil && !isFlexible() {
		log.Printf("no config exist, and no api server or interactive cli enabled, exiting...")
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
		RoutingEnv.MainFallback = mainFallback
	}

	var tproxyConfs []*proxy.ListenConf

	//load inServers and RoutingEnv
	switch mode {
	case proxy.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		defaultInServer, hase, eie = proxy.ServerFromURL(simpleConf.Server_ThatListenPort_Url)
		if hase {
			if ce := utils.CanLogErr("can not create local server"); ce != nil {
				ce.Write(zap.Error(eie))
			}
			return -1
		}

		if !defaultInServer.CantRoute() && simpleConf.Route != nil {

			netLayer.LoadMaxmindGeoipFile("")

			//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
			RoutingEnv.RoutePolicy = netLayer.NewRoutePolicy()
			if simpleConf.MyCountryISO_3166 != "" {
				RoutingEnv.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(simpleConf.MyCountryISO_3166))

			}
		}
	case proxy.StandardMode:

		RoutingEnv, Default_uuid = proxy.LoadEnvFromStandardConf(&standardConf)

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

			AllServers = append(AllServers, thisServer)
			if tag := thisServer.GetTag(); tag != "" {
				vs.ServersTagMap[tag] = thisServer
			}
		}

	}

	// load outClients
	switch mode {
	case proxy.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		DefaultOutClient, hase, eie = proxy.ClientFromURL(simpleConf.Client_ThatDialRemote_Url)
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
			AllClients = append(AllClients, thisClient)

			if tag := thisClient.GetTag(); tag != "" {
				vs.ClientsTagMap[tag] = thisClient
			}
		}

		if len(AllClients) > 0 {
			DefaultOutClient = AllClients[0]

		} else {
			DefaultOutClient = vs.DirectClient
		}

	}

	if DefaultOutClient != nil && len(tproxyConfs) > 0 {
		for _, thisConf := range tproxyConfs {
			tm := vs.ListenTproxy(thisConf.GetAddrStrForListenOrDial(), DefaultOutClient)
			if tm != nil {
				TproxyList = append(TproxyList, tm)
			}

		}
	}

	configFileQualifiedToRun := false

	if (defaultInServer != nil || len(AllServers) > 0 || len(TproxyList) > 0) && (DefaultOutClient != nil) {
		configFileQualifiedToRun = true

		if mode == proxy.SimpleMode {
			lis := vs.ListenSer(defaultInServer, DefaultOutClient, &RoutingEnv)
			if lis != nil {
				ListenerArray = append(ListenerArray, lis)
			}
		} else {
			for _, inServer := range AllServers {
				lis := vs.ListenSer(inServer, DefaultOutClient, &RoutingEnv)

				if lis != nil {
					ListenerArray = append(ListenerArray, lis)
				}
			}
		}

	}
	//没配置可用的listen或者dial，而且还无法动态更改配置
	if !configFileQualifiedToRun && !isFlexible() {
		utils.Error("No valid proxy settings available, exit now.")
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

func cleanup() {
	//在程序ctrl+C关闭时, 会主动Close所有的监听端口. 主要是被报告windows有时退出程序之后, 端口还是处于占用状态.
	// 用下面代码以试图解决端口占用问题.

	for _, listener := range ListenerArray {
		if listener != nil {
			listener.Close()
		}
	}

	for _, tm := range TproxyList {
		tm.Stop()
	}
}
