//go:build windows

package app

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/tyler-builds/jamb/internal/platform/windows"
)

// isSID reports whether s is a user SID in its string form: S-1- followed by two
// or more groups of decimal digits separated by hyphens, such as S-1-5-21-...
// The SID is written into a security descriptor string, so it is checked first.
func isSID(s string) bool {
	rest, ok := strings.CutPrefix(s, "S-1-")
	if !ok {
		return false
	}
	groups := strings.Split(rest, "-")
	if len(groups) < 2 {
		return false
	}
	for _, g := range groups {
		if g == "" {
			return false
		}
		for i := 0; i < len(g); i++ {
			if g[i] < '0' || g[i] > '9' {
				return false
			}
		}
	}
	return true
}

// currentInstanceNames returns the instance names of appID for the current user in
// the current session.
func currentInstanceNames(appID string) (instanceNames, error) {
	u, err := user.Current()
	if err != nil {
		return instanceNames{}, err
	}
	if !isSID(u.Uid) {
		return instanceNames{}, fmt.Errorf("the user identifier %q is not a SID", u.Uid)
	}
	session, err := windows.SessionID()
	if err != nil {
		return instanceNames{}, err
	}
	mutex, pipe := instanceNamesFor(appID, u.Uid, session)
	return instanceNames{mutex: mutex, pipe: pipe, sid: u.Uid}, nil
}

// claimInstance creates the mutex, and reports whether this process is the first
// instance.
func claimInstance(mutex string) (bool, error) { return windows.ClaimInstance(mutex) }

// servePipe serves the instance's pipe; see windows.ServePipe.
func servePipe(names instanceNames, serve func(conn io.ReadWriter)) error {
	return windows.ServePipe(names.pipe, names.sid, serve)
}

// dialHandoff connects to the first instance's pipe, and returns the process ID of
// the first instance.
func dialHandoff(pipe string, timeout time.Duration) (io.ReadWriteCloser, uint32, error) {
	p, pid, err := windows.DialPipe(pipe, timeout)
	if err != nil {
		return nil, 0, err
	}
	return p, pid, nil
}

// allowForeground lets the first instance bring its windows to the front.
func allowForeground(pid uint32) { windows.AllowForegroundWindow(pid) }

// setAppID sets the process's Application User Model ID. A packaged process keeps
// the identity of its package, which Windows uses in place of an explicit ID, so
// nothing is set for it.
func setAppID(id string) error {
	if windows.PackagedProcess() {
		return nil
	}
	return windows.SetAppUserModelID(id)
}

// commitJumpList sets the process's jump list from j. Each task starts this
// program's executable.
func commitJumpList(j *JumpList) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tasks := make([]windows.ShellTask, len(j.Tasks))
	for i, t := range j.Tasks {
		path, index := parseIcon(t.Icon)
		tasks[i] = windows.ShellTask{
			Title: t.Title, Path: exe, Args: t.Args, Description: t.Description,
			IconPath: path, IconIndex: index,
		}
	}
	return windows.SetJumpList(tasks, j.Recent)
}

// addRecentDocument adds the absolute path to the Recent documents.
func addRecentDocument(path string) { windows.AddRecentDocument(path) }
