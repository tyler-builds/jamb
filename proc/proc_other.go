//go:build !windows

package proc

import "os/exec"

// hide does nothing: only Windows opens a console window for a program.
func hide(*exec.Cmd) {}
