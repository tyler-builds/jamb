package main

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW, the process creation flag that starts a console
// program without a console window.
const createNoWindow = 0x08000000

// hideProcess makes cmd start without a console window, and asks the program to keep
// its first window hidden, in case it opens one.
func hideProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
