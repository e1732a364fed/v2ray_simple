package utils

import (
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// Stdout, Stderr to zap
func LogRunCmd(name string, arg ...string) (out string, err error) {
	ZapLogger.Info("run cmd", zap.String("cmd", name), zap.Strings("args", arg))

	cmd1 := exec.Command(name, arg...)
	var sbE strings.Builder
	var sbO strings.Builder
	cmd1.Stderr = &sbE
	cmd1.Stdout = &sbO

	if err = cmd1.Run(); err != nil {
		ZapLogger.Error("run cmd failed", zap.Error(err))
	}
	out = sbO.String()
	ZapLogger.Info("run cmd result", zap.String("stdOut", out), zap.String("stdErr", sbE.String()))

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

	for _, str := range strs {
		if err = ExecCmd(str); err != nil {
			return
		}
	}

	return
}
