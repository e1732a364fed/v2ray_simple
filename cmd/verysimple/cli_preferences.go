//go:build !nocli

package main

import (
	"fmt"

	"github.com/e1732a364fed/v2ray_simple/utils"
)

const (
	openAutoRearrangeStr = "开启自动将最近运行的交互命令提升到首位"
)

var (
	toggleAutoRearrangeCliCmd = &CliCmd{Name: openAutoRearrangeStr}
)

type CliPreference struct {
	CliCmdOrder []int `toml:"cli_cmd_order"`
	AutoArrange bool  `toml:"auto_arrange"`
}

func init() {
	cliCmdList = append(cliCmdList, toggleAutoRearrangeCliCmd)

	toggleAutoRearrangeCliCmd.f = func() {
		cp := currentUserPreference.Cli
		if cp == nil {
			cp = new(CliPreference)
			currentUserPreference.Cli = cp
		}

		cp.AutoArrange = !cp.AutoArrange
		doWhenUpdateAutoRearrangeCli()

	}

	preference_loadFunclist = append(preference_loadFunclist, loadPreferences_cli)
}

func doWhenUpdateAutoRearrangeCli() {

	cp := currentUserPreference.Cli
	if cp == nil {
		cp = new(CliPreference)
		currentUserPreference.Cli = cp
	}

	if !cp.AutoArrange {
		toggleAutoRearrangeCliCmd.Name = openAutoRearrangeStr

	} else {

		if len(cp.CliCmdOrder) == 0 {
			initOrderList()
		}
		toggleAutoRearrangeCliCmd.Name = "关闭自动将最近运行的交互命令提升到首位"
	}

}

func initOrderList() {
	cp := currentUserPreference.Cli
	if cp == nil {
		cp = new(CliPreference)
		currentUserPreference.Cli = cp
	}
	cp.CliCmdOrder = nil
	for i := range cliCmdList {
		cp.CliCmdOrder = append(cp.CliCmdOrder, i)
	}

}

func updateMostRecentCli(i int) {

	cp := currentUserPreference.Cli
	if cp == nil {
		cp = new(CliPreference)
		currentUserPreference.Cli = cp
	}

	utils.MoveItem(&cliCmdList, i, 0)
	utils.MoveItem(&cp.CliCmdOrder, i, 0)
}

func loadPreferences_cli() {
	cp := currentUserPreference.Cli
	if cp == nil {
		return
	}

	doWhenUpdateAutoRearrangeCli()
	if len(cp.CliCmdOrder) == 0 {
		return
	}
	if len(cp.CliCmdOrder) <= len(cliCmdList) {
		var neworder []int
		var ei int
		cliCmdList, neworder, ei = utils.SortByOrder(cliCmdList, cp.CliCmdOrder)
		if ei != 0 {
			fmt.Println("utils.SortByOrder got ei", ei)
		}
		if neworder != nil {
			cp.CliCmdOrder = neworder
		}

	} else {
		initOrderList()
	}

}
