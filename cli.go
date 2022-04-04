package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/hahahrfool/v2ray_simple/proxy"
	"github.com/hahahrfool/v2ray_simple/utils"
	"github.com/manifoldco/promptui"
)

func init() {
	cliCmdList = append(cliCmdList, CliCmd{
		"交互生成配置，超级强大", func() {
			generateConfigFileInteractively()
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

var cliCmdList []CliCmd

//交互式命令行用户界面
//
//阻塞，可按ctrl+C推出
func runCli() {
	defer func() {
		fmt.Printf("Interactive Mode exited. \n")
		if ce := utils.CanLogInfo("Interactive Mode exited"); ce != nil {
			ce.Write()
		}
	}()

	langList := []string{"Chinese", "English"}
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

	for {
		Select = promptui.Select{
			Label: "请选择想执行的功能",
			Items: cliCmdList,
		}

		i, result, err := Select.Run()

		if err != nil {
			fmt.Printf("Prompt failed %v\n", err)
			return
		}

		fmt.Printf("你选择了 %q\n", result)

		if f := cliCmdList[i].F; f != nil {
			f()
		}
	}

}

func generateConfigFileInteractively() {

	rootLevelList := []string{
		"打印当前生成的配置",
		"配置app配置 (即全局配置)",
		"添加dns (可不填）",
		"添加dial (拨号）",
		"添加listen (监听）",
		"添加route (分流，可不填）",
		"添加fallback (回落，可不填）",
	}

	configStruct := proxy.Standard{}

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

		fmt.Printf("你选择了 %q\n", result)

		switch i {
		case 0: //print
			buf := utils.GetBuf()
			if err := toml.NewEncoder(buf).Encode(configStruct); err != nil {
				// failed to encode
				log.Fatal(err)
			}
			fmt.Println(buf.String())

		case 1: //app
			if configStruct.App == nil {
				configStruct.App = &proxy.AppConf{}
			}

			appList := []string{
				"配置 loglevel",
				"配置 DefaultUUID",
				"配置 mycountry",
				"配置 noreadv",
				"配置 admin_pass",
			}

			subselect := promptui.Select{
				Label: "请选择要执行的子项",
				Items: appList,
			}

			i, result, err := subselect.Run()

			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			fmt.Printf("你选择了 %q\n", result)

			switch i {
			case 0: //loglevel
				var theint64 int64
				validate := func(input string) error {
					var err error
					theint64, err = strconv.ParseInt(input, 10, 64)
					if err != nil {
						return errors.New("Invalid number")
					}
					return nil
				}

				prompt := promptui.Prompt{
					Label:    "Number",
					Validate: validate,
				}

				result, err := prompt.Run()

				if err != nil {
					fmt.Printf("验证失败 failed %v\n", err)
					return
				}

				fmt.Printf("你输入了 %q\n", result)
				ii := int(theint64)
				configStruct.App.LogLevel = &ii
			case 1:

			case 2:
			case 3:
			case 4:

			}

		case 2: //dns
		case 3: //dial
		case 4: //listen
		case 5: //route
		case 6: //fallback
		}
	}
}
