// Command jamb-notes is a note-taking app built with jamb.
package main

import (
	"fmt"
	"os"

	"github.com/tyler-builds/jamb/app"

	"github.com/tyler-builds/jamb/examples/notes/store"
	"github.com/tyler-builds/jamb/examples/notes/views"
)

func main() {
	path := store.DefaultPath()
	s, err := store.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb-notes:", err)
		os.Exit(1)
	}
	settingsPath := store.SettingsPath(path)
	settings := store.LoadSettings(settingsPath)
	app.New().Run(app.Window{
		Title:     "Jamb Notes",
		Width:     1100,
		Height:    720,
		MinWidth:  720,
		MinHeight: 420,
		Root:      views.Root(s),
		Bindings:  views.Bindings(),
		// keymap.json next to the notes file overrides the shortcuts.
		KeymapFile: store.KeymapPath(path),
		Actions:    views.Actions(),
		// Problems in keymap.json show in the status bar.
		OnKeymapLoad: views.ReportKeymap,
		// Ctrl+=/ Ctrl+- / Ctrl+0 and Ctrl+wheel zoom; remember the choice.
		Zoom: settings.Zoom,
		OnZoom: func(z float32) {
			settings.Zoom = z
			_ = store.SaveSettings(settingsPath, settings)
		},
		// Pending edits are written as the window closes; there is no prompt.
		OnClose: func() bool {
			if err := s.Flush(); err != nil {
				fmt.Fprintln(os.Stderr, "jamb-notes: saving before close failed:", err)
			}
			return true
		},
	})
	if err := s.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "jamb-notes: saving on exit failed:", err)
		os.Exit(1)
	}
}
