//go:build windows

package app

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tyler-builds/jamb/internal/platform/windows"
)

// stampFlag is a hidden command-line flag for the installer. The NSIS installer runs the
// installed program once with the flag and the AppID, after it has created the program's
// Start menu shortcut. The program then sets the AppID on that shortcut and exits in init,
// before its main function, any window, or any single-instance handoff. Applications do
// not use the flag, and it is not part of the API.
const stampFlag = "--jamb-stamp-shortcut"

func init() {
	if len(os.Args) != 3 || os.Args[1] != stampFlag {
		return
	}
	os.Exit(stampShortcutMain(os.Args[2]))
}

// stampShortcutMain runs the stamp flag and returns the exit status: 0 when the program's
// shortcut carries the AppID afterwards, and 1 otherwise, with the reason on stderr.
func stampShortcutMain(appID string) int {
	windows.InitializeCOM()
	if appID == "" {
		fmt.Fprintln(os.Stderr, "jamb:", stampFlag, "needs an AppID")
		return 1
	}
	if err := stampShortcut(appID); err != nil {
		fmt.Fprintln(os.Stderr, "jamb:", err)
		return 1
	}
	return 0
}

// stampShortcut sets appID on the program's Start menu shortcut, when registration would
// set it. It creates nothing: when no shortcut to the program can take the AppID, it is an
// error.
func stampShortcut(appID string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	user, err := userPrograms()
	if err != nil {
		return err
	}
	action, path := chooseShortcut(appID, exe, "", findShortcuts(startMenuRoots(user)))
	switch action {
	case shortcutKeep:
		return nil
	case shortcutStamp:
		return windows.SetShortcutAppID(path, appID)
	}
	return fmt.Errorf("no Start menu shortcut to %s can be given the AppID %s", exe, appID)
}

// userPrograms returns the user's Start menu Programs folder.
func userPrograms() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "Microsoft", "Windows", "Start Menu", "Programs"), nil
}

// startMenuRoots returns the folders that registration searches for shortcuts: the user's
// Programs folder first, then the all-users one, when the system has one.
func startMenuRoots(user string) []string {
	roots := []string{user}
	if pd := os.Getenv("ProgramData"); pd != "" {
		roots = append(roots, filepath.Join(pd, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return roots
}

// findShortcuts returns the shortcut files (.lnk) in the roots and their subfolders, in
// search order, with what registration needs about each. A shortcut that cannot be read
// is skipped. It needs COM on the calling thread: the UI thread has it, and so does the
// stamp flag, which initializes it.
func findShortcuts(roots []string) []shortcutCandidate {
	var found []shortcutCandidate
	for _, root := range roots {
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			target, appID, err := windows.ReadShortcut(path)
			if err != nil {
				return nil
			}
			found = append(found, shortcutCandidate{Path: path, Target: target, AppID: appID, Writable: writable(path)})
			return nil
		})
	}
	return found
}

// writable reports whether the file at path can be opened for writing. Nothing is
// written, so the file keeps its contents and its time stamp.
func writable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	return f.Close() == nil
}
