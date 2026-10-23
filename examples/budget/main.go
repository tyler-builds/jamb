// Command jamb-budget keeps a household's budget: accounts, transactions sorted
// into categories, a monthly amount for each category, and how the month is going
// against it. It imports transactions from the CSV files banks export, and shows
// money in the currency of Windows' region settings.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tyler-builds/jamb/app"
	// The visual inspector: Ctrl+Shift+I opens it (see App.DevTools).
	_ "github.com/tyler-builds/jamb/devtools"
	"github.com/tyler-builds/jamb/ui"

	"github.com/tyler-builds/jamb/examples/budget/ledger"
	"github.com/tyler-builds/jamb/examples/budget/views"
)

// settings are the app's preferences, kept next to the ledger.
type settings struct {
	Theme string  `json:"theme,omitempty"`
	Zoom  float32 `json:"zoom,omitempty"`
}

func main() {
	path := ledger.DefaultPath()
	d, ok, err := ledger.Load(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "jamb-budget: the ledger cannot be read:", err)
		os.Exit(1)
	}
	if !ok {
		// The first run starts from a made-up household, so there is something to see.
		d = ledger.Sample(time.Now())
	}
	s := ledger.New(d, path)

	settingsPath := filepath.Join(filepath.Dir(path), "settings.json")
	var st settings
	if b, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	saveSettings := func() {
		if b, err := json.MarshalIndent(st, "", "  "); err == nil {
			_ = os.MkdirAll(filepath.Dir(settingsPath), 0o755)
			_ = os.WriteFile(settingsPath, b, 0o644)
		}
	}

	a := views.New(s, views.Options{Theme: st.Theme, OnTheme: func(theme string) {
		st.Theme = theme
		saveSettings()
	}})
	w := app.Window{
		Title:     "Jamb Budget",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 560,
		Chrome:    app.ChromeCustom,
		Root:      a.Root(),
		Bindings:  views.Bindings(),
		Actions:   views.Actions(),
		Zoom:      st.Zoom,
		OnZoom: func(z float32) {
			st.Zoom = z
			saveSettings()
		},
		OnClose: func() bool {
			if err := s.Flush(); err != nil {
				fmt.Fprintln(os.Stderr, "jamb-budget: saving the ledger failed:", err)
			}
			return true
		},
	}
	switch st.Theme {
	case "light":
		w.Theme = ui.LightTheme()
	case "dark":
		w.Theme = ui.DarkTheme()
	}
	app.New().AppID("JambExamples.Budget").DevTools(true).Run(w)
}
