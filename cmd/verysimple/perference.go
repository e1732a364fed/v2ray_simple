//go:build !nocli || gui

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

/*
用户偏好功能, 在有cli/gui时自动启用：
1. 自动将最近运行的交互命令提升到首位; 退出交互模式时将顺序记录到文件; 进入交互模式时加载文件并加载回顺序
2. （todo）记录用户所选择的语言
3. （todo）记录每个tag的listen 和 dial 所消耗的流量
*/

const (
	preferencesFileName = ".verysimple_preferences"
)

var (
	disablePreferenceFeature bool
	currentUserPreference    UserPreference

	preference_loadFunclist []func()
)

type UserPreference struct {
	Cli *CliPreference `toml:"cli"`
	Gui *GuiPreference `toml:"gui"`
}

func init() {
	flag.BoolVar(&disablePreferenceFeature, "dp", false, "if given, vs won't save your interactive mode preferences.")

}

func savePerferences() {
	if disablePreferenceFeature {
		return
	}
	fmt.Println("Saving preferences")

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

	fmt.Println("Loading preferences")

	bs, err := os.ReadFile(preferencesFileName)
	if err != nil {
		fmt.Println("Failed loading preferences file,", err)
		return
	}
	err = toml.Unmarshal(bs, &currentUserPreference)
	if err != nil {
		fmt.Println("Failed Unmarshal preferences toml,", err)
		return
	}

	for _, f := range preference_loadFunclist {
		f()
	}
}
