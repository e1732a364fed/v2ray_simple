//go:build !nocli

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	vs "github.com/e1732a364fed/v2ray_simple"
	"github.com/e1732a364fed/v2ray_simple/machine"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

var interactive_mode bool

type CliCmd struct {
	Name string
	f    func()
}

func (cc CliCmd) String() string {
	return cc.Name
}

// func nlist(list []CliCmd) (result []string) {
// 	for _, v := range list {
// 		result = append(result, v.Name)
// 	}
// 	return
// }

func flist(list []*CliCmd) (result []func()) {
	for _, v := range list {
		result = append(result, v.f)
	}
	return
}

// cliCmdList 包含所有交互模式中可执行的命令；
// 本文件 中添加的 CliCmd都是直接返回运行结果的、无需进一步交互的命令
var cliCmdList = []*CliCmd{
	{
		"查询当前状态", func() {
			defaultMachine.PrintAllState(os.Stdout)
		},
	}, {
		"打印当前版本所支持的所有协议", printSupportedProtocols,
	}, {
		"生成随机ssl证书", generateRandomSSlCert,
	}, {
		"生成一个随机的uuid供你参考", generateAndPrintUUID,
	}, {
		"下载geosite文件夹", tryDownloadGeositeSource,
	}, {
		"下载geoip文件(GeoLite2-Country.mmdb)", tryDownloadMMDB,
	},
}

func init() {
	flag.BoolVar(&interactive_mode, "i", false, "enable interactive commandline mode")

	//cli.go 中添加的 CliCmd都是需进一步交互的命令

	cliCmdList = append(cliCmdList, &CliCmd{
		"【生成分享链接】<-当前的配置", func() {
			sc := defaultMachine.GetStandardConfFromCurrentState()
			interactively_generate_share(&sc)
		},
	}, &CliCmd{
		"【交互生成配置】，超级强大", func() { generateConfigFileInteractively(defaultMachine) },
	}, &CliCmd{
		"热删除配置", func() { interactively_hotRemoveServerOrClient(defaultMachine) },
	}, &CliCmd{
		"【热加载】新配置文件", func() { interactively_hotLoadConfigFile(defaultMachine) },
	}, &CliCmd{
		"【热加载】新配置url", func() { interactively_hotLoadUrlConfig(defaultMachine) },
	}, &CliCmd{
		"调节日志等级", interactively_adjust_loglevel,
	})

	runCli = runCli_func
}

// 交互式命令行用户界面
//
// 阻塞，可按ctrl+C退出或回退到上一级
func runCli_func() {
	defer func() {
		utils.PrintStr("Interactive Mode exited. \n")
		if ce := utils.CanLogInfo("Interactive Mode exited"); ce != nil {
			ce.Write()
		}

		savePerferences()
	}()

	loadPreferences()

	/*
		langList := []string{"简体中文", "English"}
		utils.PrintStr("Welcome to Interactive Mode, please choose a Language \n")
		Select := promptui.Select{
			Label: "Select Language",
			Items: langList,
		}

		_, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("You choose %q\n", result)

		if result != langList[0] {
			utils.PrintStr("Sorry, language not supported yet \n")
			return
		}
	*/

	for {
		Select := promptui.Select{
			Label: "请选择想执行的功能",
			Items: cliCmdList,
		}

		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你选择了 %s\n", result)

		if f := cliCmdList[i].f; f != nil {
			f()
		}

		if currentUserPreference.AutoArrange {
			updateMostRecentCli(i)
		}

	}

}

func generateConfigFileInteractively(m *machine.M) {

	rootLevelList := []string{
		"【打印】当前缓存的配置",
		"【开始交互生成】配置",
		"【清除】此次缓存的配置",
		"【写到文件】<-将该缓存的配置 (client.toml和 server.toml)",
		"【生成分享】链接url <-该缓存的配置",
		"【投入运行】（热加载) <-将此次生成的配置",
	}

	confClient := proxy.StandardConf{}
	confServer := proxy.StandardConf{}

	var clientStr, serverStr string

	for {
		Select := promptui.Select{
			Label: "请选择想为你的配置文件做的事情",
			Items: rootLevelList,
		}

		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你选择了 %s\n", result)

		generateConfStr := func() {

			confClient.Route = []*netLayer.RuleConf{{
				DialTag: "direct",
				Domains: []string{"geosite:cn"},
			}}

			var vsConfClient machine.VSConf = machine.VSConf{
				AppConf:      &machine.AppConf{MyCountryISO_3166: "CN"},
				StandardConf: confClient,
			}

			clientStr, err = utils.GetPurgedTomlStr(&vsConfClient)
			if err != nil {
				log.Fatal(err)
			}

			serverStr, err = utils.GetPurgedTomlStr(confServer)
			if err != nil {
				log.Fatal(err)
			}
		}

		switch i {
		case 0: //print

			generateConfStr()

			buf := utils.GetBuf()

			buf.WriteString("#客户端配置\n")
			buf.WriteString(clientStr)
			buf.WriteString("\n")

			buf.WriteString("#服务端配置\n")
			buf.WriteString(serverStr)
			buf.WriteString("\n")
			utils.PrintStr(buf.String())

			utils.PutBuf(buf)

		case 1:
			interactively_generateConf(&confClient, &confServer)

		case 2: //clear
			confClient = proxy.StandardConf{}
			confServer = proxy.StandardConf{}
			clientStr = ""
			serverStr = ""
		case 3: //output

			generateConfStr()

			var clientFile *os.File
			clientFile, err = os.OpenFile("client.toml", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println("Can't create client.toml", err)
				return
			}
			clientFile.WriteString(clientStr)
			clientFile.Close()

			var serverFile *os.File
			serverFile, err = os.OpenFile("server.toml", os.O_WRONLY|os.O_CREATE, 0666)
			if err != nil {
				fmt.Println("Can't create server.toml", err)
				return
			}
			serverFile.WriteString(serverStr)
			serverFile.Close()

			utils.PrintStr("生成成功！请查看文件\n")
		case 4: //share url
			if len(confClient.Dial) > 0 {

				utils.PrintStr("生成的分享链接如下：\n")

				interactively_generate_share(&confClient)

			} else {
				utils.PrintStr("请先进行配置\n")

			}
		case 5: //hot load
			utils.PrintStr("因为本次同时生成了服务端和客户端配置, 请选择要热加载的是哪一个\n")
			selectHot := promptui.Select{
				Label: "加载客户端配置还是服务端配置？",
				Items: []string{
					"服务端",
					"客户端",
				},
			}
			ihot, result, err := selectHot.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你选择了 %s\n", result)

			switch ihot {
			case 0:

				m.HotLoadDialConf("", confServer.Dial)
				m.HotLoadListenConf(confServer.Listen)

			case 1:
				m.HotLoadDialConf("", confClient.Dial)
				m.HotLoadListenConf(confClient.Listen)
			}

			utils.PrintStr("加载成功！你可以回退(ctrl+c)到上级来使用 【查询当前状态】来查询新增的配置\n")

		} // switch i case 1
	} //for
}

// 热删除配置
func interactively_hotRemoveServerOrClient(m *machine.M) {
	utils.PrintStr("即将开始热删除配置步骤, 删除正在运行的配置可能有未知风险，谨慎操作\n")
	utils.PrintStr("【当前所有配置】为：\n")
	utils.PrintStr(delimiter)
	m.PrintAllState(os.Stdout)

	var items []string
	if len(m.AllServers) > 0 {
		items = append(items, "listen")
	}
	if len(m.AllClients) > 0 {
		items = append(items, "dial")
	}
	if len(items) == 0 {
		utils.PrintStr("没listen也没dial! 一个都没有，没法删!\n")
		return
	}

	Select := promptui.Select{
		Label: "请选择你想删除的是dial还是listen",
		Items: items,
	}

	i, result, err := Select.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	var will_delete_listen, will_delete_dial bool

	var will_delete_index int

	fmt.Printf("你选择了 %s\n", result)
	switch i {
	case 0:
		if len(m.AllServers) > 0 {
			will_delete_listen = true

		} else if len(m.AllClients) > 0 {
			will_delete_dial = true
		}

	case 1:
		if len(m.AllServers) > 0 {
			will_delete_dial = true

		}
	}

	var theInt int64

	if (will_delete_dial && len(m.AllClients) > 1) || (will_delete_listen && len(m.AllServers) > 1) {

		validateFunc := func(input string) error {
			theInt, err = strconv.ParseInt(input, 10, 64)
			if err != nil || theInt < 0 {
				return utils.ErrInvalidNumber
			}

			if will_delete_dial && int(theInt) >= len(m.AllClients) {
				return errors.New("must with in len of dial array")
			}

			if will_delete_listen && int(theInt) >= len(m.AllServers) {
				return errors.New("must with in len of listen array")
			}

			return nil
		}

		utils.PrintStr("请输入你想删除的序号\n")

		promptIdx := promptui.Prompt{
			Label:    "序号",
			Validate: validateFunc,
		}

		_, err = promptIdx.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你输入了 %d\n", theInt)

	}

	will_delete_index = int(theInt)

	if will_delete_dial {
		m.HotDeleteClient(will_delete_index)
	}
	if will_delete_listen {
		m.HotDeleteServer(will_delete_index)

	}

	utils.PrintStr("删除成功！当前状态：\n")
	utils.PrintStr(delimiter)
	m.PrintAllState(os.Stdout)
}

func interactively_hotLoadUrlConfig(m *machine.M) {
	utils.PrintStr("即将开始热添加url配置\n")
	Select := promptui.Select{
		Label: "请选择你的url的格式类型",
		Items: []string{
			"vs标准url格式",
			"协议官方url格式(视代理协议不同而不同)",
		},
	}
	i, result, err := Select.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	fmt.Printf("你选择了 %s\n", result)

	switch i {
	case 1:
		fmt.Printf("目前暂不支持")
		return

	case 0:
		Select := promptui.Select{
			Label: "请选择该url是用于dial还是listen",
			Items: []string{
				"dial",
				"listen",
			},
		}
		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}
		fmt.Printf("你选择了 %s\n", result)

		fmt.Printf("请输入你的配置url\n")

		var theUrlStr string

		fmt.Scanln(&theUrlStr)

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		if i == 0 {
			m.HotLoadDialUrl(theUrlStr, proxy.UrlFormat)

		} else {
			m.HotLoadListenUrl(theUrlStr, proxy.UrlFormat)

		}
		return

	}
}

// 热添加配置文件
func interactively_hotLoadConfigFile(m *machine.M) {
	utils.PrintStr("即将开始热添加配置文件\n")
	utils.PrintStr("【注意】我们交互模式只支持热添加listen和dial, 对于dns/route/fallback的热增删, 请期待api server未来的实现.\n")
	utils.PrintStr("【当前所有配置】为：\n")
	utils.PrintStr(delimiter)
	m.PrintAllState(os.Stdout)

	utils.PrintStr("请输入你想添加的文件名称\n")

	promptFile := promptui.Prompt{
		Label: "配置文件",
		Validate: func(s string) error {

			if err := utils.IsFilePath(s); err != nil {
				return err
			}
			if !utils.FileExist(utils.GetFilePath(s)) {
				return os.ErrNotExist
			}
			return nil
		},
	}

	fpath, err := promptFile.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	fmt.Printf("你输入了 %s\n", fpath)

	var confMode int
	var simpleConf proxy.SimpleConf
	confMode, simpleConf, _, err = LoadConfig(fpath, "", "")
	if err != nil {

		log.Printf("can not load standard config file: %s\n", err)
		return
	}

	//listen, dial, dns, route, fallbacks 这几项都可以选择性加载

	//但是route和fallback的话，动态增删很麻烦，因为route/fallback可能配置相当多条;

	//而dns的话,没法简单增删, 而是会覆盖。

	//因此我们交互模式暂且只支持 listen和dial的热加载。 dns/route/fallback的热增删可以用apiServer实现.

	//也就是说，理论上要写一个比较好的前端，才能妥善解决 复杂条目的热增删问题。

	switch confMode {
	case proxy.StandardMode:
		if len(standardConf.Dial) > 0 {
			defaultMachine.HotLoadDialConf("", standardConf.Dial)

		}

		if len(standardConf.Listen) > 0 {
			defaultMachine.HotLoadListenConf(standardConf.Listen)

		}
	case proxy.SimpleMode:
		r, ser := defaultMachine.LoadSimpleServer(simpleConf)
		if r < 0 {
			return
		}

		r, cli := defaultMachine.LoadSimpleClient(simpleConf)
		if r < 0 {
			return
		}

		lis := vs.ListenSer(ser, cli, &m.RoutingEnv)
		if lis != nil {
			m.ListenCloserList = append(m.ListenCloserList, lis)
		}

	}

	utils.PrintStr("添加成功！当前状态：\n")
	utils.PrintStr(delimiter)
	m.PrintAllState(os.Stdout)
}

func interactively_adjust_loglevel() {
	fmt.Println("当前日志等级为：", utils.LogLevelStr(utils.LogLevel))

	list := utils.LogLevelStrList()
	Select := promptui.Select{
		Label: "请选择你调节为点loglevel",
		Items: list,
	}

	i, result, err := Select.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	fmt.Printf("你选择了 %s\n", result)

	if i < len(list) && i >= 0 {
		utils.LogLevel = i
		utils.InitLog("")

		utils.PrintStr("调节 日志等级完毕. 现在等级为\n")
		utils.PrintStr(list[i])
		utils.PrintStr("\n")

	}
}
