// Package proc starts other programs from an app without flashing console windows.
//
// An app packaged for Windows is a GUI program (built with -H=windowsgui), so it has no
// console. When it starts a console program, such as git, Windows opens a new console
// window for that program, which shows for as long as the program runs: an app that runs
// git every few seconds flashes a window every few seconds. Command and CommandContext
// return an exec.Cmd that starts the program with a console but no window. The programs
// it starts in turn share that hidden console, so they open no window either. On other
// systems they are exec.Command and exec.CommandContext.
//
// Use them for a console program that runs in the background, whose output the app reads.
// A program that should show a window, such as a terminal the user asked for or an
// editor, needs exec.Command and creation flags of its own: a GUI program started by
// Command keeps its first window hidden.
package proc

import (
	"context"
	"os/exec"
)

// Command is exec.Command for a background console program: on Windows the program
// starts without a console window.
func Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	Hide(cmd)
	return cmd
}

// CommandContext is exec.CommandContext for a background console program: on Windows
// the program starts without a console window.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...)
	Hide(cmd)
	return cmd
}

// Hide makes cmd start without a console window on Windows, and asks the program to
// keep its first window hidden. It keeps cmd's other SysProcAttr settings, but drops
// flags that would give the program a visible console or none at all. It does nothing
// on other systems. Call it before cmd.Start.
func Hide(cmd *exec.Cmd) { hide(cmd) }
