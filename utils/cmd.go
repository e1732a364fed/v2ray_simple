package utils

import (
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// Stdout, Stderr to zap
func LogRunCmd(name string, arg ...string) (out string, err error) {
	if ce := CanLogInfo("run cmd"); ce != nil {
		ce.Write(zap.String("cmd", name), zap.Strings("args", arg))
	}

	cmd1 := exec.Command(name, arg...)
	var sbE strings.Builder
	var sbO strings.Builder
	cmd1.Stderr = &sbE
	cmd1.Stdout = &sbO

	if err = cmd1.Run(); err != nil {

		if ce := CanLogErr("run cmd failed"); ce != nil {
			ce.Write(zap.Error(err), zap.String("stdOut", out), zap.String("stdErr", sbE.String()))
		}

	}
	out = sbO.String()

	if ce := CanLogInfo("run cmd"); ce != nil {
		ce.Write(zap.String("stdOut", out), zap.String("stdErr", sbE.String()))
	}

	return
}

// Stdout, Stderr to fmt
func FmtPrintRunCmd(name string, arg ...string) (out string, err error) {
	fmt.Println("run cmd", "cmd", name, "args", arg)

	cmd1 := exec.Command(name, arg...)
	var sbE strings.Builder
	var sbO strings.Builder
	cmd1.Stderr = &sbE
	cmd1.Stdout = &sbO

	if err = cmd1.Run(); err != nil {
		fmt.Println("run cmd failed", err, "stdOut", out, "stdErr", sbE.String())
	}
	out = sbO.String()
	fmt.Println("run cmd result", "stdOut:", out, "stdErr:", sbE.String())

	return
}

func ExecCmd(cmdStr string) (err error) {
	ZapLogger.Info("run cmd", zap.String("cmd", cmdStr))

	strs := strings.Split(cmdStr, " ")

	cmd1 := exec.Command(strs[0], strs[1:]...)
	if err = cmd1.Run(); err != nil {
		ZapLogger.Error("run cmd failed", zap.Error(err))
	}

	return
}

func ExecCmdMultilineList(cmdStr string) (err error) {

	strs := strings.Split(cmdStr, "\n")

	err = ExecCmdList(strs)

	return
}

func ExecCmdList(strs []string) (err error) {

	for i, str := range strs {
		if err = ExecCmd(str); err != nil {
			err = NumErr{N: i, E: err}
			return
		}
	}

	return
}

func LogExecCmdList(strs []string) (err error) {

	for i, str := range strs {
		ss := strings.Split(str, " ")
		if _, err = LogRunCmd(ss[0], ss[1:]...); err != nil {
			err = NumErr{N: i, E: err}
			return
		}
	}

	return
}
