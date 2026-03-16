package main

import (
	"os/exec"
	"runtime"
)

var openBrowserCmd = defaultOpenBrowserCmd

func openBrowser(url string) error {
	name, args := openBrowserCmd(url)
	return exec.Command(name, args...).Start()
}

func defaultOpenBrowserCmd(url string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{url}
	case "windows":
		return "cmd", []string{"/c", "start", "", url}
	default:
		return "xdg-open", []string{url}
	}
}
