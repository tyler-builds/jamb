//go:build !windows

package app

import (
	"errors"
	"io"
	"time"
)

// errNotWindows is returned by the identity and single instance calls on platforms
// without a window adapter. Such a run fails in runPlatform anyway.
var errNotWindows = errors.New("single instance and taskbar identity are supported on Windows only")

func currentInstanceNames(string) (instanceNames, error) {
	return instanceNames{}, errNotWindows
}

func claimInstance(string) (bool, error) { return true, nil }

func servePipe(instanceNames, func(conn io.ReadWriter)) error { return errNotWindows }

func dialHandoff(string, time.Duration) (io.ReadWriteCloser, uint32, error) {
	return nil, 0, errNotWindows
}

func allowForeground(uint32) {}

func setAppID(string) error { return nil }

func commitJumpList(*JumpList) error { return nil }

func addRecentDocument(string) {}
