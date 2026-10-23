package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tyler-builds/jamb/ui"
)

// Settings are the app's preferences and the arrangement of its window.
type Settings struct {
	// IntervalMS is the time between samples, in milliseconds.
	IntervalMS int `json:"intervalMs,omitempty"`
	// Theme is "light", "dark" or "" to follow Windows.
	Theme string  `json:"theme,omitempty"`
	Zoom  float32 `json:"zoom,omitempty"`
	// Layout is the arrangement of the panels, nil for the default.
	Layout *ui.DockLayout `json:"layout,omitempty"`
	// Columns and Sort are the process table's columns and sort.
	Columns ui.ColumnState `json:"columns,omitzero"`
	Sort    *ui.SortState  `json:"sort,omitempty"`
	Rules   *Rules         `json:"rules,omitempty"`
	// CloseToTray hides the window to the notification area when it is closed.
	CloseToTray bool `json:"closeToTray,omitempty"`
	// Notify shows an OS notification for each alert.
	Notify *bool `json:"notify,omitempty"`
	// Grouped groups the process table by program.
	Grouped *bool `json:"grouped,omitempty"`
}

// SettingsPath is where the settings are kept: jamb-monitor\settings.json in the
// user's configuration folder.
func SettingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "jamb-monitor", "settings.json")
}

// LoadSettings reads settings; a missing or unreadable file gives defaults.
func LoadSettings(path string) Settings {
	var s Settings
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

// SaveSettings writes settings, creating the directory if needed.
func SaveSettings(path string, s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
