package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/pprof"
	"strings"
	"syscall"

	vs "github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/pkg/profile"
	"go.uber.org/zap"
)

var (
	configFileName string
	standardConf   vs.StandardConf

	startPProf bool
	startMProf bool
)

func init() {
	flag.StringVar(&configFileName, "c", "client.toml", "config file name")
	flag.BoolVar(&startPProf, "pp", false, "pprof")
	flag.BoolVar(&startMProf, "mp", false, "memory pprof")

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

	utils.ShouldLogToFile = true

	utils.InitLog()

	if startPProf {
		f, _ := os.OpenFile("cpu.pprof", os.O_CREATE|os.O_RDWR, 0644)
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()

	}
	if startMProf {
		//若不使用 NoShutdownHook, 我们ctrl+c退出时不会产生 pprof文件
		p := profile.Start(profile.MemProfile, profile.MemProfileRate(1), profile.NoShutdownHook)

		defer p.Stop()
	}

	utils.Info("Program started")
	defer utils.Info("Program exited")
	var err error
	if standardConf, err = vs.LoadConfig(configFileName); err != nil && !isFlexible() {
		log.Printf("no config exist, and no api server or interactive cli enabled, exiting...")
		return -1
	}

	netLayer.Prepare()

	fmt.Printf("Log Level:%d\n", utils.LogLevel)
	fmt.Printf("UseReadv:%t\n", netLayer.UseReadv)
	fmt.Printf("tls_lazy_encrypt:%t\n", vs.Tls_lazy_encrypt)

	runPreCommands()

	var defaultInServer proxy.Server

	//load inServers and vs.RoutePolicy
	switch vs.ConfMode {
	case vs.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		defaultInServer, hase, eie = proxy.ServerFromURL(vs.SimpleConf.Server_ThatListenPort_Url)
		if hase {
			if ce := utils.CanLogErr("can not create local server"); ce != nil {
				ce.Write(zap.Error(eie))
			}
			return -1
		}

		if !defaultInServer.CantRoute() && vs.SimpleConf.Route != nil {

			netLayer.LoadMaxmindGeoipFile("")

			//极简模式只支持通过 mycountry进行 geoip分流 这一种情况
			vs.RoutePolicy = netLayer.NewRoutePolicy()
			if vs.SimpleConf.MyCountryISO_3166 != "" {
				vs.RoutePolicy.AddRouteSet(netLayer.NewRouteSetForMyCountry(vs.SimpleConf.MyCountryISO_3166))

			}
		}
	case vs.StandardMode:

		vs.LoadCommonComponentsFromStandardConf(&standardConf)

		//虽然标准模式支持多个Server，目前先只考虑一个
		//多个Server存在的话，则必须要用 tag指定路由; 然后，我们需在预先阶段就判断好tag指定的路由

		if len(standardConf.Listen) < 1 {
			utils.Warn("no listen in config settings")
			break
		}

		for _, serverConf := range standardConf.Listen {
			thisConf := serverConf
			if thisConf.Protocol == "tproxy" {
				vs.ListenTproxy(thisConf.GetAddrStrForListenOrDial())
				continue
			}

			if thisConf.Uuid == "" && vs.Default_uuid != "" {
				thisConf.Uuid = vs.Default_uuid
			}

			thisServer, err := proxy.NewServer(thisConf)
			if err != nil {
				if ce := utils.CanLogErr("can not create local server:"); ce != nil {
					ce.Write(zap.Error(err))
				}
				continue
			}

			vs.AllServers = append(vs.AllServers, thisServer)
			if tag := thisServer.GetTag(); tag != "" {
				vs.ServersTagMap[tag] = thisServer
			}
		}

	}

	// load outClients
	switch vs.ConfMode {
	case vs.SimpleMode:
		var hase bool
		var eie utils.ErrInErr
		vs.DefaultOutClient, hase, eie = proxy.ClientFromURL(vs.SimpleConf.Client_ThatDialRemote_Url)
		if hase {
			if ce := utils.CanLogErr("can not create remote client"); ce != nil {
				ce.Write(zap.Error(eie))
			}
			return -1
		}
	case vs.StandardMode:

		if len(standardConf.Dial) < 1 {
			utils.Warn("no dial in config settings")
			break
		}

		for _, thisConf := range standardConf.Dial {
			if thisConf.Uuid == "" && vs.Default_uuid != "" {
				thisConf.Uuid = vs.Default_uuid
			}

			thisClient, err := proxy.NewClient(thisConf)
			if err != nil {
				if ce := utils.CanLogErr("can not create remote client: "); ce != nil {
					ce.Write(zap.Error(err))
				}
				continue
			}
			vs.AllClients = append(vs.AllClients, thisClient)

			if tag := thisClient.GetTag(); tag != "" {
				vs.ClientsTagMap[tag] = thisClient
			}
		}

		if len(vs.AllClients) > 0 {
			vs.DefaultOutClient = vs.AllClients[0]

		} else {
			vs.DefaultOutClient = vs.DirectClient
		}

	}

	configFileQualifiedToRun := false

	if (defaultInServer != nil || len(vs.AllServers) > 0 || len(vs.TproxyList) > 0) && (vs.DefaultOutClient != nil) {
		configFileQualifiedToRun = true

		if vs.ConfMode == vs.SimpleMode {
			vs.ListenSer(defaultInServer, vs.DefaultOutClient, true)
		} else {
			for _, inServer := range vs.AllServers {
				vs.ListenSer(inServer, vs.DefaultOutClient, true)
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

		//在程序ctrl+C关闭时, 会主动Close所有的监听端口. 主要是被报告windows有时退出程序之后, 端口还是处于占用状态.
		// 用下面代码以试图解决端口占用问题.

		for _, listener := range vs.ListenerArray {
			if listener != nil {
				listener.Close()
			}
		}

		for _, tm := range vs.TproxyList {
			tm.Stop()
		}

	}
	return
}
