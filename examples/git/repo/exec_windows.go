package repo

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps git from opening a console window when the app is built
// as a GUI program, which has no console of its own to share.
const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
