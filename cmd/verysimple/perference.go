//go:build !nocli || gui

package main

import (
	"flag"
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

	preference_saveFunclist []func()
	preference_loadFunclist []func()
)

type UserPreference struct {
	CliCmdOrder []int `toml:"cli_cmd_order"`
	AutoArrange bool  `toml:"auto_arrange"`
}

func init() {
	flag.BoolVar(&disablePreferenceFeature, "dp", false, "if given, vs won't save your interactive mode preferences.")

}

func savePerferences() {
	for _, f := range preference_saveFunclist {
		f()
	}
}

func loadPreferences() {
	for _, f := range preference_loadFunclist {
		f()
	}
}
