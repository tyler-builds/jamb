// Command jamb-git is a Git history browser built with jamb: several
// repositories, their commit graph, branches and tags, and each commit's changes.
// It reads repositories with the git command, which must be on PATH, and never
// changes them.
package main

import (
	"os"
	"sync"
	"time"

	"github.com/tyler-builds/jamb/app"
	// The visual inspector: Ctrl+Shift+I opens it (see App.DevTools).
	_ "github.com/tyler-builds/jamb/devtools"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/git/repo"
	"github.com/tyler-builds/jamb/examples/git/store"
	"github.com/tyler-builds/jamb/examples/git/views"
)

func main() {
	path := store.SettingsPath()
	settings := store.LoadSettings(path)
	repos := settings.Repos
	// On the first run, start with the repository the app was started in, if any.
	if len(repos) == 0 {
		if wd, err := os.Getwd(); err == nil {
			if info, err := repo.Open(wd); err == nil {
				repos = []string{info.Root}
			}
		}
	}
	s := store.New(repos, settings.Current)
	if s.Current.Peek() == "" && len(repos) > 0 {
		s.Current.Set(repos[0])
	}
	sidebar := ui.NewSignal(orDefault(settings.SidebarWidth, views.DefaultSidebarWidth))
	detail := ui.NewSignal(orDefault(settings.DetailWidth, views.DefaultDetailWidth))
	theme := ui.NewSignal(settings.Theme)

	// save runs on the UI thread and from the save timer's goroutine.
	var mu sync.Mutex
	save := func() {
		mu.Lock()
		defer mu.Unlock()
		settings.Repos = s.Paths()
		settings.Current = s.Current.Peek()
		settings.SidebarWidth = sidebar.Peek()
		settings.DetailWidth = detail.Peek()
		settings.Theme = theme.Peek()
		_ = store.SaveSettings(path, settings)
	}
	// Each change restarts the timer, so a drag of a divider is saved once it pauses.
	var timer *time.Timer
	changed := func() {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(400*time.Millisecond, save)
	}

	w := app.Window{
		Title:      "Jamb Git",
		Width:      1360,
		Height:     840,
		MinWidth:   960,
		MinHeight:  540,
		Chrome:     app.ChromeCustom,
		LightTheme: views.LightTheme,
		DarkTheme:  views.DarkTheme,
		Root:       views.Root(s, views.Layout{Sidebar: sidebar, Detail: detail, OnChange: changed}, theme),
		Bindings:   views.Bindings(),
		Actions:    views.Actions(),
		Zoom:       settings.Zoom,
		OnZoom: func(z float32) {
			mu.Lock()
			settings.Zoom = z
			mu.Unlock()
			changed()
		},
		OnClose: func() bool {
			save()
			return true
		},
	}
	switch settings.Theme {
	case "light":
		w.Theme = views.LightTheme
	case "dark":
		w.Theme = views.DarkTheme
	}
	app.New().AppID("JambExamples.Git").DevTools(true).Run(w)
}

func orDefault(v float64, def int) float64 {
	if v <= 0 {
		return float64(def)
	}
	return v
}
