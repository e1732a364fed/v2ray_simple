package utils

import (
	"log"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//https://github.com/golang/go/issues/20455#issuecomment-342287698
func fixAndroidTimezone() {
	out, err := exec.Command("/system/bin/getprop", "persist.sys.timezone").Output()
	if err != nil {
		log.Println("fixAndroidTimezone failed when calling /system/bin/getprop,", err)
		return
	}
	z, err := time.LoadLocation(strings.TrimSpace(string(out)))
	if err != nil {
		log.Println("fixAndroidTimezone failed,", err)
		return
	}
	time.Local = z
}

func init() {
	if runtime.GOOS == "android" {
		fixAndroidTimezone()
	}
}

func IsTimezoneCN() bool {
	_, offset := time.Now().Zone()
	return offset/3600 == 8
}
