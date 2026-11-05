// Command jamb-notes is a note-taking app built with jamb.
package main

import (
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/app"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/examples/notes/views"
)

// closeErr is the error of the write as the window closed, which Unsaved
// reports; Save runs off the UI thread, so it is guarded.
var closeErr lockedErr

type lockedErr struct {
	mu  sync.Mutex
	err error
}

func (l *lockedErr) set(err error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.err = err
	return err
}

func (l *lockedErr) get() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// newNoteArg is the argument of the "New note" task in the jump list.
const newNoteArg = "--new-note"

func main() {
	path := store.DefaultPath()
	s, err := store.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb-notes:", err)
		os.Exit(1)
	}
	// notes.list, notes.create and the others, for coding agents (jamb ctl run).
	views.RegisterCommands(s)
	settingsPath := store.SettingsPath(path)
	settings := store.LoadSettings(settingsPath)
	// The sidebar's width is kept with the other settings. It is saved shortly after
	// the divider stops moving, and as the window closes.
	sidebarWidth := settings.SidebarWidth
	if sidebarWidth <= 0 {
		sidebarWidth = views.DefaultSidebarWidth
	}
	sidebar := ui.NewSignal(sidebarWidth)
	// saveSettings runs on the UI thread and from the save timer's goroutine.
	var settingsMu sync.Mutex
	saveSettings := func() {
		settingsMu.Lock()
		defer settingsMu.Unlock()
		settings.SidebarWidth = sidebar.Peek()
		_ = store.SaveSettings(settingsPath, settings)
	}
	// Each move of the divider restarts the timer, so a drag is saved once it pauses.
	var sidebarSave *time.Timer
	onSidebarChange := func(float64) {
		if sidebarSave != nil {
			sidebarSave.Stop()
		}
		sidebarSave = time.AfterFunc(400*time.Millisecond, saveSettings)
	}
	root := views.RootWith(s, views.Layout{
		Sidebar: sidebar, OnSidebarChange: onSidebarChange, OnImport: app.AddRecentDocument,
	})
	// A launch from the "New note" task makes its note once its window has rendered.
	// A launch that hands its command line to a running app never gets a window, so
	// it makes no note of its own.
	startWithNote := slices.Contains(os.Args[1:], newNoteArg)
	// One process serves every launch: a second launch hands its command line to
	// the running app, which makes the note here when the task asked for one.
	app.New().
		AppID("JambExamples.Notes").
		SingleInstance(func(args []string, _ string) {
			if slices.Contains(args, newNoteArg) {
				s.Create()
			}
		}).
		JumpList(app.JumpList{
			Tasks:  []app.JumpTask{{Title: "New note", Args: newNoteArg, Description: "Create an empty note"}},
			Recent: true,
		}).
		Run(app.Window{
			Title:     "Jamb Notes",
			Width:     1100,
			Height:    720,
			MinWidth:  720,
			MinHeight: 420,
			Root: func(ctx *ui.Context) ui.Element {
				if startWithNote {
					startWithNote = false
					ctx.Post(func() { s.Create() })
				}
				return root(ctx)
			},
			Bindings: views.Bindings(),
			// keymap.json next to the notes file overrides the shortcuts.
			KeymapFile: store.KeymapPath(path),
			Actions:    views.Actions(),
			// Problems in keymap.json show in the status bar.
			OnKeymapLoad: views.ReportKeymap,
			// Ctrl+=/ Ctrl+- / Ctrl+0 and Ctrl+wheel zoom; remember the choice.
			Zoom: settings.Zoom,
			OnZoom: func(z float32) {
				settingsMu.Lock()
				settings.Zoom = z
				settingsMu.Unlock()
				saveSettings()
			},
			// Pending edits are written as the window closes; there is no prompt,
			// unless that write fails: then Unsaved reports the notes, and the
			// window asks to save them again or to discard them.
			OnClose: func() bool {
				saveSettings()
				if err := closeErr.set(s.Flush()); err != nil {
					fmt.Fprintln(os.Stderr, "jamb-notes: saving before close failed:", err)
				}
				return true
			},
			Unsaved: func() *app.Changes {
				if closeErr.get() == nil {
					return nil
				}
				return &app.Changes{Save: func() error { return closeErr.set(s.Flush()) }}
			},
		})
	if err := s.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "jamb-notes: saving on exit failed:", err)
		os.Exit(1)
	}
}
