package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/e1732a364fed/v2ray_simple/netLayer"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/proxy/trojan"
	"github.com/e1732a364fed/v2ray_simple/proxy/vless"
	"github.com/e1732a364fed/v2ray_simple/tlsLayer"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

var cliCmdList = []CliCmd{
	{
		"生成随机ssl证书", func() {
			const certFn = "cert.pem"
			const keyFn = "cert.key"
			if utils.FileExist(certFn) {
				fmt.Printf(certFn)
				fmt.Printf(" 已存在！\n")
				return
			}

			if utils.FileExist(keyFn) {
				fmt.Printf(keyFn)
				fmt.Printf(" 已存在！\n")
				return
			}

			err := tlsLayer.GenerateRandomCertKeyFiles(certFn, keyFn)
			if err == nil {
				fmt.Printf("生成成功！请查看目录中的 ")
				fmt.Printf(certFn)
				fmt.Printf(" 和 ")
				fmt.Printf(keyFn)
				fmt.Printf("\n")

			} else {

				fmt.Printf("生成失败,")
				fmt.Printf(err.Error())
				fmt.Printf("\n")

			}
		},
	},
}

func init() {

	//cli.go 中定义的 CliCmd都是需进一步交互的命令

	cliCmdList = append(cliCmdList, CliCmd{
		"交互生成配置，超级强大", func() {
			generateConfigFileInteractively()
		},
	})
	cliCmdList = append(cliCmdList, CliCmd{
		"热删除配置", func() {
			interactively_hotRemoveServerOrClient()
		},
	})
	cliCmdList = append(cliCmdList, CliCmd{
		"热加载新配置文件", func() {
			interactively_hotLoadConfigFile()
		},
	})
	cliCmdList = append(cliCmdList, CliCmd{
		"调节日志等级", func() {
			interactively_adjust_loglevel()
		},
	})

}

type CliCmd struct {
	Name string
	F    func()
}

func (cc CliCmd) String() string {
	return cc.Name
}

//交互式命令行用户界面
//
//阻塞，可按ctrl+C退出或回退到上一级
func runCli() {
	defer func() {
		fmt.Printf("Interactive Mode exited. \n")
		if ce := utils.CanLogInfo("Interactive Mode exited"); ce != nil {
			ce.Write()
		}
	}()

	/*
		langList := []string{"简体中文", "English"}
		fmt.Printf("Welcome to Interactive Mode, please choose a Language \n")
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
			fmt.Printf("Sorry, language not supported yet \n")
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

		if f := cliCmdList[i].F; f != nil {
			f()
		}
	}

}

func generateConfigFileInteractively() {

	rootLevelList := []string{
		"打印当前缓存的配置",
		"开始交互生成配置",
		"清除此次缓存的配置",
		"将该缓存的配置写到文件(client.toml和 server.toml)",
		"以该缓存的配置【生成客户端分享链接url】",
		"将此次生成的配置投入运行（热加载）",
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

			confClient.App = &proxy.AppConf{MyCountryISO_3166: "CN"}

			clientStr, err = utils.GetPurgedTomlStr(confClient)
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

			fmt.Printf("#客户端配置\n")
			fmt.Printf(clientStr)
			fmt.Printf("\n")

			fmt.Printf("#服务端配置\n")
			fmt.Printf(serverStr)
			fmt.Printf("\n")

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

			fmt.Println("生成成功！请查看文件")
		case 4: //share url
			if len(confClient.Dial) > 0 {

				fmt.Println("生成的分享链接如下：")

				for _, d := range confClient.Dial {
					switch d.Protocol {
					case vless.Name:
						fmt.Println(vless.GenerateXrayShareURL(d))

					case trojan.Name:
						fmt.Println(trojan.GenerateOfficialDraftShareURL(d))
					}
				}

			} else {
				fmt.Println("请先进行配置")

			}
		case 5: //hot load
			fmt.Println("因为本次同时生成了服务端和客户端配置, 请选择要热加载的是哪一个")
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

				hotLoadDialConfForRuntime(confServer.Dial)
				hotLoadListenConfForRuntime(confServer.Listen)

			case 1:
				hotLoadDialConfForRuntime(confClient.Dial)
				hotLoadListenConfForRuntime(confClient.Listen)
			}

			fmt.Printf("加载成功！你可以回退(ctrl+c)到上级来使用 【查询当前状态】来查询新增的配置\n")

		case 1: //interactively generate

			select0 := promptui.Select{
				Label: "【提醒】我们交互模式生成的配置都是直接带tls的,且客户端【默认使用utls】模拟chrome指纹",
				Items: []string{"知道了"},
			}

			_, _, err := select0.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			select2 := promptui.Select{
				Label: "请选择你客户端想监听的协议",
				Items: []string{
					"socks5",
					"http",
				},
			}
			i2, result, err := select2.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你选择了 %s\n", result)

			if i2 < 2 {
				confClient.Listen = append(confClient.Listen, &proxy.ListenConf{})
			} else {
				fmt.Printf("Prompt failed, werid input")
				return
			}

			clientlisten := confClient.Listen[0]
			clientlisten.Protocol = result
			clientlisten.Tag = "my_" + result

			var theInt int64

			var canLowPort bool
			validatePort := func(input string) error {
				theInt, err = strconv.ParseInt(input, 10, 64)
				if err != nil {
					return errors.New("Invalid number")
				}
				if !canLowPort {
					if theInt <= 1024 {
						return errors.New("Invalid number")
					}
				}
				if theInt > 65535 {
					return errors.New("Invalid number")
				}
				return nil
			}

			fmt.Printf("请输入你客户端想监听的端口\n")

			promptPort := promptui.Prompt{
				Label:    "Port Number",
				Validate: validatePort,
			}

			result, err = promptPort.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你输入了 %d\n", theInt)

			clientlisten.Port = int(theInt)
			clientlisten.IP = "127.0.0.1"

			select3 := promptui.Select{
				Label: "请选择你客户端想拨号的协议(与服务端监听协议相同)",
				Items: []string{
					"vless",
					"trojan",
				},
			}
			i3, result, err := select3.Run()

			if err != nil || i3 != 0 {
				fmt.Println("Prompt failed ", err, i3)
				return
			}

			fmt.Printf("你选择了 %s\n", result)
			theProtocol := result

			confClient.Dial = append(confClient.Dial, &proxy.DialConf{})
			clientDial := confClient.Dial[0]

			fmt.Printf("请输入你服务端想监听的端口\n")
			canLowPort = true

			result, err = promptPort.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你输入了 %d\n", theInt)

			clientDial.Port = int(theInt)
			clientDial.Protocol = theProtocol
			clientDial.TLS = true
			clientDial.Tag = "my_proxy"
			clientDial.Utls = true

			select4 := promptui.Select{
				Label: "请选择你客户端拨号想使用的高级层(与服务端监听的高级层相同)",
				Items: []string{
					"无",
					"ws",
					"grpc",
					"quic",
				},
			}
			i4, result, err := select4.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}

			switch i4 {
			case 0:
			default:
				clientDial.AdvancedLayer = result
				switch i4 {
				case 1, 2:
					clientlisten.Tag += "_" + result
					promptPath := promptui.Prompt{
						Label: "Path",
						Validate: func(s string) error {
							if result == "ws" && !strings.HasPrefix(s, "/") {
								return errors.New("ws path must start with /")
							}
							return nil
						},
					}

					result, err = promptPath.Run()
					if err != nil {
						fmt.Println("Prompt failed ", err, result)
						return
					}

					fmt.Printf("你输入了 %s\n", result)

					clientDial.Path = result

				}
			}

			fmt.Printf("请输入你服务端的ip\n")

			promptIP := promptui.Prompt{
				Label:    "IP",
				Validate: utils.WrapFuncForPromptUI(govalidator.IsIP),
			}

			result, err = promptIP.Run()
			if err != nil {
				fmt.Println("Prompt failed ", err, result)
				return
			}

			fmt.Printf("你输入了 %s\n", result)

			clientDial.IP = result

			fmt.Printf("请输入你服务端的域名\n")

			promptDomain := promptui.Prompt{
				Label:    "域名",
				Validate: func(s string) error { return nil }, //允许不设域名
			}

			result, err = promptDomain.Run()
			if err != nil {
				fmt.Println("Prompt failed ", err, result)
				return
			}

			fmt.Printf("你输入了 %s\n", result)

			clientDial.Host = result

			select5 := promptui.Select{
				Label: "请选择uuid生成方式",
				Items: []string{
					"随机",
					"手动输入(要保证你输入的是格式正确的uuid)",
				},
			}
			i5, result, err := select5.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}
			if i5 == 0 {
				uuid := utils.GenerateUUIDStr()
				clientDial.Uuid = uuid
				fmt.Println("随机生成的uuid为", uuid)
			} else {
				promptUUID := promptui.Prompt{
					Label:    "uuid",
					Validate: utils.WrapFuncForPromptUI(govalidator.IsUUID),
				}

				result, err = promptUUID.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				clientDial.Uuid = result
			}

			var serverListenStruct proxy.ListenConf
			serverListenStruct.CommonConf = clientDial.CommonConf
			serverListenStruct.IP = "0.0.0.0"

			confServer.Listen = append(confServer.Listen, &serverListenStruct)

			confServer.Dial = append(confServer.Dial, &proxy.DialConf{
				CommonConf: proxy.CommonConf{
					Protocol: "direct",
				},
			})

			serverListen := confServer.Listen[0]

			select6 := promptui.Select{
				Label: "请配置服务端tls证书路径",
				Items: []string{
					"默认(cert.pem和cert.key),此时将自动开启 insecure",
					"手动输入(要保证你输入的是正确的文件路径)",
				},
			}
			i6, result, err := select6.Run()

			if err != nil {
				fmt.Println("Prompt failed ", err, i3)
				return
			}
			if i6 == 0 {
				serverListen.TLSCert = "cert.pem"
				serverListen.TLSKey = "cert.key"
				serverListen.Insecure = true
				clientDial.Insecure = true

				fmt.Printf("你选择了默认自签名证书, 这是不安全的, 我们不推荐. 所以自动生成证书这一步需要你一会再到交互模式里选择相应选项进行生成。 \n")

			} else {
				fmt.Printf("请输入 cert路径\n")

				promptCPath := promptui.Prompt{
					Label:    "path",
					Validate: utils.IsFilePath,
				}

				result, err = promptCPath.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				serverListen.TLSCert = result

				fmt.Printf("请输入 key 路径\n")

				result, err = promptCPath.Run()
				if err != nil {
					fmt.Println("Prompt failed ", err, result)
					return
				}

				fmt.Printf("你输入了 %s\n", result)

				serverListen.TLSKey = result
			}

		} // switch i case 1
	} //for
}

//热删除配置
func interactively_hotRemoveServerOrClient() {
	fmt.Printf("即将开始热删除配置步骤, 删除正在运行的配置可能有未知风险，谨慎操作\n")
	fmt.Printf("【当前所有配置】为：\n")
	fmt.Printf(delimiter)
	printAllState(os.Stdout, true)

	var items []string
	if len(allServers) > 0 {
		items = append(items, "listen")
	}
	if len(allClients) > 0 {
		items = append(items, "dial")
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
		will_delete_listen = true
	case 1:
		will_delete_dial = true
	}

	var theInt int64

	if (will_delete_dial && len(allClients) > 1) || (will_delete_listen && len(allServers) > 1) {

		validateFunc := func(input string) error {
			theInt, err = strconv.ParseInt(input, 10, 64)
			if err != nil || theInt < 0 {
				return errors.New("Invalid number")
			}

			if will_delete_dial && int(theInt) >= len(allClients) {
				return errors.New("must with in len of dial array")
			}

			if will_delete_listen && int(theInt) >= len(allServers) {
				return errors.New("must with in len of listen array")
			}

			return nil
		}

		fmt.Printf("请输入你想删除的序号\n")

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
		doomedClient := allClients[will_delete_index]

		routingEnv.DelClient(doomedClient.GetTag())
		doomedClient.Stop()
		allClients = utils.TrimSlice(allClients, will_delete_index)
	}
	if will_delete_listen {
		listenerArray[will_delete_index].Close()
		allServers[will_delete_index].Stop()

		allServers = utils.TrimSlice(allServers, will_delete_index)
		listenerArray = utils.TrimSlice(listenerArray, will_delete_index)

	}

	fmt.Printf("删除成功！当前状态：\n")
	fmt.Printf(delimiter)
	printAllState(os.Stdout, true)
}

//热添加配置文件
func interactively_hotLoadConfigFile() {
	fmt.Printf("即将开始热添加配置文件\n")
	fmt.Printf("【注意】我们交互模式只支持热添加listen和dial, 对于dns/route/fallback的热增删, 请期待api server未来的实现.\n")
	fmt.Printf("【当前所有配置】为：\n")
	fmt.Printf(delimiter)
	printAllState(os.Stdout, false)

	fmt.Printf("请输入你想添加的文件名称\n")

	promptFile := promptui.Prompt{
		Label: "配置文件",
		Validate: func(s string) error {

			if err := utils.IsFilePath(s); err != nil {
				return err
			}
			if !utils.FileExist(utils.GetFilePath(s)) {
				return errors.New("文件不存在")
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

	standardConf, err = proxy.LoadTomlConfFile(fpath)
	if err != nil {

		log.Printf("can not load standard config file: %s\n", err)
		return
	}

	//listen, dial, dns, route, fallbacks 这几项都可以选择性加载

	//但是route和fallback的话，动态增删很麻烦，因为route/fallback可能配置相当多条;

	//而dns的话,没法简单增删, 而是会覆盖。

	//因此我们交互模式暂且只支持 listen和dial的热加载。 dns/route/fallback的热增删可以用apiServer实现.

	//也就是说，理论上要写一个比较好的前端，才能妥善解决 复杂条目的热增删问题。

	if len(standardConf.Dial) > 0 {
		hotLoadDialConfForRuntime(standardConf.Dial)

	}

	if len(standardConf.Listen) > 0 {
		hotLoadListenConfForRuntime(standardConf.Listen)

	}

	fmt.Printf("添加成功！当前状态：\n")
	fmt.Printf(delimiter)
	printAllState(os.Stdout, false)
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

		fmt.Printf("调节 日志等级完毕. 现在等级为\n")
		fmt.Printf(list[i])
		fmt.Printf("\n")

	}
}
