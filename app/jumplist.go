package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// JumpList is the app's Windows jump list: the tasks in its taskbar menu, and
// whether the menu has a Recent category. The app sets it with JumpList, when it
// starts, under its AppID. Nothing is listed unless the app sets it, and the
// framework adds no tasks of its own.
//
// Each task starts the app's own executable with Args, as a new process. Without
// SingleInstance, that process opens its own window and keeps its own state. With
// SingleInstance, the new process hands its command line to the running app, which
// receives it in onArgs. Use SingleInstance when a task should act in the running
// app, such as opening a window in it, rather than start another one.
type JumpList struct {
	// Tasks are listed under Tasks, in order. Windows may show only the first few.
	Tasks []JumpTask
	// Recent shows the Recent category, which lists the documents given to
	// AddRecentDocument.
	Recent bool
}

// JumpTask is one task of a JumpList.
type JumpTask struct {
	// Title is the text the menu shows. It is required.
	Title string
	// Args is the command line after the executable, such as "--new-note". Windows
	// takes it as one string, so an argument with spaces needs quotes, as in a
	// shell.
	Args string
	// Description is shown as the tooltip of the task.
	Description string
	// Icon is an optional path and icon index, as "path,index", such as
	// "C:\\app\\icons.dll,2". A comma followed by a number is read as the index;
	// otherwise the whole string is the path, with index 0. Empty uses the
	// executable's own icon.
	Icon string
}

// JumpList sets the app's jump list, which is committed when the app starts. It
// needs an AppID. Each task starts the program's executable, and a program run
// with go run has an executable that is removed when the run ends, so its tasks
// stop working then. See the README's Jump lists and single instance section.
func (a *App) JumpList(j *JumpList) *App {
	a.jumpList = j
	return a
}

// check reports a task that cannot be shown, before the app starts.
func (j *JumpList) check() error {
	for i, t := range j.Tasks {
		if strings.TrimSpace(t.Title) == "" {
			return fmt.Errorf("app.JumpList: task %d has no Title", i+1)
		}
	}
	return nil
}

// parseIcon splits an icon of the form "path,index". The index is the number after
// the last comma; when there is none, or the text after the comma is not a number,
// the whole string is the path, and the index is 0.
func parseIcon(icon string) (path string, index int) {
	if i := strings.LastIndex(icon, ","); i >= 0 {
		if n, err := strconv.Atoi(icon[i+1:]); err == nil {
			return icon[:i], n
		}
	}
	return icon, 0
}

// AddRecentDocument adds the file at path to the Recent category of the app's
// jump list, which the app shows when JumpList.Recent is set. Call it for each
// file the app opens or saves. A relative path is made absolute first, because
// Windows needs the full path. Windows keeps the list and drops old entries itself.
// It does nothing on platforms other than Windows.
func AddRecentDocument(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb: AddRecentDocument:", err)
		return
	}
	addRecentDocument(abs)
}
