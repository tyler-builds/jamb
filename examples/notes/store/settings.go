package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are app preferences kept next to the notes file.
type Settings struct {
	Zoom float32 `json:"zoom,omitempty"`
	// SidebarWidth is the width of the sidebar in logical pixels, as the divider left it.
	SidebarWidth float64 `json:"sidebarWidth,omitempty"`
}

// SettingsPath returns the settings file that goes with a notes file.
func SettingsPath(notesPath string) string {
	return filepath.Join(filepath.Dir(notesPath), "settings.json")
}

// KeymapPath returns the keymap file that goes with a notes file. Its
// bindings override the built-in shortcuts.
func KeymapPath(notesPath string) string {
	return filepath.Join(filepath.Dir(notesPath), "keymap.json")
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
