package app

import (
	"testing"

	"github.com/tyler-builds/jamb/ui"
)

// A window without a Theme follows the app mode with its pair (the built-in
// themes by default); a Theme pins it.
func TestWindowThemeChoice(t *testing.T) {
	w := withDefaults(Window{})
	if w.themeFor(true) != w.DarkTheme || w.themeFor(false) != w.LightTheme || w.DarkTheme == nil || w.LightTheme == nil {
		t.Fatal("a window without a Theme does not follow the app mode with the built-in pair")
	}
	pinned := ui.DarkTheme()
	w = withDefaults(Window{Theme: pinned})
	if w.themeFor(false) != pinned || w.themeFor(true) != pinned {
		t.Fatal("a pinned Theme is not used in both app modes")
	}
}
