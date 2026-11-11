package proc

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
)

func TestHideSetsFlags(t *testing.T) {
	for _, cmd := range []*exec.Cmd{Command("git", "status"), CommandContext(context.Background(), "git")} {
		a := cmd.SysProcAttr
		if a == nil || !a.HideWindow || a.CreationFlags&createNoWindow == 0 {
			t.Errorf("SysProcAttr = %+v, want HideWindow and CREATE_NO_WINDOW", a)
		}
	}
}

func TestHideKeepsOtherSettings(t *testing.T) {
	const createNewProcessGroup = 0x00000200
	cmd := exec.Command("git")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup | createNewConsole | detachedProcess, NoInheritHandles: true}
	Hide(cmd)
	a := cmd.SysProcAttr
	if !a.NoInheritHandles || a.CreationFlags != createNewProcessGroup|createNoWindow || !a.HideWindow {
		t.Errorf("SysProcAttr = %+v, want the process group and NoInheritHandles kept, consoles swapped for CREATE_NO_WINDOW", a)
	}
}
