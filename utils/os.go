package utils

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
)

func OpenFile(name string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", name).Start()

	case "windows":
		err = exec.Command("cmd", "/C start "+name).Start()
	case "darwin":
		err = exec.Command("open", name).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err

}

// https://gist.github.com/hyg/9c4afcd91fe24316cbf0
func Openbrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	return err

}

func GetSystemKillChan() <-chan os.Signal {
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM) //os.Kill cannot be trapped
	return osSignals
}
