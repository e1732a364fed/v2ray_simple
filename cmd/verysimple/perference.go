//go:build !nocli

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

/*
用户偏好功能：
1. 自动将最近运行的交互命令提升到首位; 退出交互模式时将顺序记录到文件; 进入交互模式时加载文件并加载回顺序
2. （todo）记录用户所选择的语言
3. （todo）记录每个tag的listen 和 dial 所消耗的流量
*/

const (
	preferencesFileName  = ".verysimple_preferences"
	openAutoRearrangeStr = "开启自动将最近运行的交互命令提升到首位"
)

var (
	disablePreferenceFeature  bool
	currentUserPreference     UserPreference
	toggleAutoRearrangeCliCmd = &CliCmd{Name: openAutoRearrangeStr}
)

type UserPreference struct {
	CliCmdOrder []int `toml:"cli_cmd_order"`
	AutoArrange bool  `toml:"auto_arrange"`
}

func init() {
	flag.BoolVar(&disablePreferenceFeature, "dp", false, "if given, vs won't save your interactive mode preferences.")

	cliCmdList = append(cliCmdList, toggleAutoRearrangeCliCmd)

	toggleAutoRearrangeCliCmd.f = func() {
		currentUserPreference.AutoArrange = !currentUserPreference.AutoArrange
		doWhenUpdateAutoRearrange()
	}

}

func doWhenUpdateAutoRearrange() {
	if !currentUserPreference.AutoArrange {
		toggleAutoRearrangeCliCmd.Name = openAutoRearrangeStr

	} else {

		if len(currentUserPreference.CliCmdOrder) == 0 {
			initOrderList()
		}
		toggleAutoRearrangeCliCmd.Name = "关闭自动将最近运行的交互命令提升到首位"
	}
}

func initOrderList() {
	currentUserPreference.CliCmdOrder = nil
	for i := range cliCmdList {
		currentUserPreference.CliCmdOrder = append(currentUserPreference.CliCmdOrder, i)
	}
}

func updateMostRecentCli(i int) {
	utils.MoveItem(&cliCmdList, i, 0)
	utils.MoveItem(&currentUserPreference.CliCmdOrder, i, 0)
}

func savePerferences() {
	if disablePreferenceFeature {
		return
	}

	buf := utils.GetBuf()
	defer utils.PutBuf(buf)
	if err := toml.NewEncoder(buf).Encode(currentUserPreference); err != nil {
		fmt.Println("err encountered during saving preferences,", err)
		return
	}
	err := os.WriteFile(preferencesFileName, buf.Bytes(), os.ModePerm)
	if err != nil {
		fmt.Println("err encountered during saving preferences,", err)
		return
	}
}

func loadPreferences() {
	if disablePreferenceFeature {
		return
	}
	if !utils.FileExist(preferencesFileName) {
		return

	}
	bs, err := os.ReadFile(preferencesFileName)
	if err != nil {
		fmt.Println("err encountered during loading preferences file,", err)
		return
	}
	err = toml.Unmarshal(bs, &currentUserPreference)
	if err != nil {
		fmt.Println("err encountered during toml.Unmarshal preferences file,", err)
		return
	}
	doWhenUpdateAutoRearrange()
	if len(currentUserPreference.CliCmdOrder) <= len(cliCmdList) {
		cliCmdList = utils.SortByOrder(cliCmdList, currentUserPreference.CliCmdOrder)

	} else {
		initOrderList()
	}

}
