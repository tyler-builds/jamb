//go:build !windows

package main

import "os/exec"

// hideProcess does nothing: only Windows has console windows, and the AppID check runs
// a built app only on Windows (see canRunHere).
func hideProcess(cmd *exec.Cmd) {}
