package proc

import (
	"os/exec"
	"syscall"
)

// Process creation flags (see CreateProcess).
const (
	detachedProcess  = 0x00000008 // no console at all: each console program it starts would open a window
	createNewConsole = 0x00000010 // a new, visible console window
	createNoWindow   = 0x08000000 // a console without a window, shared by the programs it starts
)

func hide(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags = cmd.SysProcAttr.CreationFlags&^(detachedProcess|createNewConsole) | createNoWindow
}
