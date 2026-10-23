package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the app's preferences and its list of repositories.
type Settings struct {
	Repos   []string `json:"repos,omitempty"`
	Current string   `json:"current,omitempty"`
	// SidebarWidth and DetailWidth are the widths of the side panes in logical
	// pixels, as the dividers left them.
	SidebarWidth float64 `json:"sidebarWidth,omitempty"`
	DetailWidth  float64 `json:"detailWidth,omitempty"`
	// Theme is "light", "dark" or "" to follow Windows.
	Theme string  `json:"theme,omitempty"`
	Zoom  float32 `json:"zoom,omitempty"`
}

// SettingsPath is where the settings are kept: jamb-git\settings.json in the
// user's configuration folder.
func SettingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "jamb-git", "settings.json")
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
