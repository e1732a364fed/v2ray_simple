//go:build !nocli

package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	openAutoRearrangeStr = "开启自动将最近运行的交互命令提升到首位"
)

var (
	toggleAutoRearrangeCliCmd = &CliCmd{Name: openAutoRearrangeStr}
)

func init() {
	cliCmdList = append(cliCmdList, toggleAutoRearrangeCliCmd)

	toggleAutoRearrangeCliCmd.f = func() {
		currentUserPreference.AutoArrange = !currentUserPreference.AutoArrange
		doWhenUpdateAutoRearrangeCli()
	}

	preference_loadFunclist = append(preference_loadFunclist, loadPreferences_cli)
	preference_saveFunclist = append(preference_saveFunclist, savePerferences_cli)
}

func doWhenUpdateAutoRearrangeCli() {
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

func savePerferences_cli() {
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

func loadPreferences_cli() {
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
	doWhenUpdateAutoRearrangeCli()
	if len(currentUserPreference.CliCmdOrder) == 0 {
		return
	}
	if len(currentUserPreference.CliCmdOrder) <= len(cliCmdList) {
		var neworder []int
		var ei int
		cliCmdList, neworder, ei = utils.SortByOrder(cliCmdList, currentUserPreference.CliCmdOrder)
		if ei != 0 {
			fmt.Println("utils.SortByOrder got ei", ei)
		}
		if neworder != nil {
			currentUserPreference.CliCmdOrder = neworder
		}

	} else {
		initOrderList()
	}

}
