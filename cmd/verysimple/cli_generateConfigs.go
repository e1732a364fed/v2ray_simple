//go:build !nocli

package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/e1732a364fed/v2ray_simple/configAdapter"
	"github.com/e1732a364fed/v2ray_simple/proxy"
	"github.com/e1732a364fed/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

func interactively_generate_share(conf *proxy.StandardConf) {

	all := []*CliCmd{
		{
			Name: "vs标准url (可用于 极简/命令行模式, #163)",
			f: func() {
				fmt.Println("Dials:")

				for _, v := range conf.Dial {
					url := configAdapter.ToVS(&v.CommonConf, v)
					fmt.Println(url)
				}

				fmt.Println("\nListens:")

				for _, v := range conf.Listen {
					url := configAdapter.ToVS(&v.CommonConf, nil)
					fmt.Println(url)
				}
			},
		},
		{
			Name: "vs标准toml",
			f: func() {
				fmt.Println("#vs_auto_generated:")

				fmt.Println(utils.GetPurgedTomlStr(conf))

			},
		},
		{
			Name: "xray分享链接标准提案 (#716)",
			f: func() {
				for _, v := range conf.Dial {
					url := configAdapter.ToXray(v)
					fmt.Println(url)
				}
			},
		},
		{
			Name: "shadowsocks uri (SIP002)",
			f: func() {
				for _, v := range conf.Dial {
					url := configAdapter.ToSS(&v.CommonConf, nil, false, 4)
					fmt.Println(url)
				}
			},
		},
		{
			Name: "v2rayN分享链接 (vmess://base64)",
			f: func() {
				for _, v := range conf.Dial {
					url := configAdapter.ToV2rayN(v)
					fmt.Println(url)
				}
			},
		},
		{
			Name: "Quantumult X (圈叉的配置的 [server_local] 部分)",
			f: func() {
				for _, v := range conf.Dial {
					url := configAdapter.ToQX(v)
					fmt.Println(url)
				}
			},
		},
		{
			Name: "Clash (yaml配置中 proxies 部分)",
			f: func() {
				for _, v := range conf.Dial {
					url := configAdapter.ToClash(v)
					fmt.Println(url)
				}
			},
		},
	}

	select0 := promptui.Select{
		Label: "请选择你想导出的分享格式 (注意, 某些分享格式只支持特定协议或特定配置)",
		Items: all,
	}
	i1, result, err := select0.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	fmt.Printf("你选择了 %s\n", result)

	flist := flist(all)
	flist[i1]()
}

func interactively_generateConf(confClient, confServer *proxy.StandardConf) {

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
		utils.PrintStr("Prompt failed, werid input")
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
			return utils.ErrInvalidNumber
		}
		if !canLowPort {
			if theInt <= 1024 {
				return utils.ErrInvalidNumber
			}
		}
		if theInt > 65535 {
			return utils.ErrInvalidNumber
		}
		return nil
	}

	utils.PrintStr("请输入你客户端想监听的端口\n")

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
			"vmess",
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

	utils.PrintStr("请输入你服务端想监听的端口\n")
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

	utils.PrintStr("请输入你服务端的ip\n")

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

	utils.PrintStr("请输入你服务端的域名\n")

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

		utils.PrintStr("你选择了默认自签名证书, 这是不安全的, 我们不推荐. 所以自动生成证书这一步需要你一会再到交互模式里选择相应选项进行生成。 \n")

	} else {
		utils.PrintStr("请输入 cert路径\n")

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

		utils.PrintStr("请输入 key 路径\n")

		result, err = promptCPath.Run()
		if err != nil {
			fmt.Println("Prompt failed ", err, result)
			return
		}

		fmt.Printf("你输入了 %s\n", result)

		serverListen.TLSKey = result
	}

}
